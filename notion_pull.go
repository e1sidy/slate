package slate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jomei/notionapi"
)

// PullResult summarizes the outcome of a pull sync operation.
type PullResult struct {
	Updated  int
	Created  int
	Comments int
	Skipped  int
	Errors   []string
}

// PullChanges pulls changes from Notion to Slate.
// Queries for pages modified since the last sync and applies changes locally.
func (nc *NotionClient) PullChanges(ctx context.Context, store *Store) (*PullResult, error) {
	result := &PullResult{}

	// Get all sync records to find the earliest last_synced_at.
	records, err := store.ListSyncRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sync records: %w", err)
	}

	// Build a map of page_id → sync record for quick lookup.
	syncByPage := make(map[string]*NotionSyncRecord)
	for _, r := range records {
		syncByPage[r.NotionPageID] = r
	}

	// Query pages from the Notion database.
	// When user_id is configured, only pull pages assigned to that user.
	var allPages []*notionapi.Page
	var cursor notionapi.Cursor
	for {
		req := &notionapi.DatabaseQueryRequest{
			PageSize: 100,
		}
		if cursor != "" {
			req.StartCursor = cursor
		}

		// Filter by assignee if user_id is configured.
		if nc.Config.UserID != "" && nc.Config.PropertyMap.Assignee != "" {
			req.Filter = &notionapi.PropertyFilter{
				Property: nc.Config.PropertyMap.Assignee,
				People: &notionapi.PeopleFilterCondition{
					Contains: nc.Config.UserID,
				},
			}
		}

		nc.rateLimit()
		resp, err := nc.API.QueryDatabase(ctx, notionapi.DatabaseID(nc.Config.DatabaseID), req)
		if err != nil {
			return nil, fmt.Errorf("query notion database: %w", err)
		}

		for i := range resp.Results {
			allPages = append(allPages, &resp.Results[i])
		}

		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}

	// Process each page.
	for _, page := range allPages {
		pageID := string(page.ID)

		if rec, ok := syncByPage[pageID]; ok {
			// Existing synced page — check if modified since last sync.
			if !page.LastEditedTime.After(rec.LastSyncedAt) {
				result.Skipped++
				continue
			}
			// Pull updates.
			if err := nc.pullUpdate(ctx, store, page, rec); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: update: %v", rec.TaskID, err))
				continue
			}
			result.Updated++
		} else {
			// New page not in sync table — create Slate task.
			if err := nc.pullCreate(ctx, store, page); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("page %s: create: %v", pageID, err))
				continue
			}
			result.Created++
		}

		// Pull comments.
		rec, _ := store.GetSyncRecordByPage(ctx, pageID)
		if rec != nil {
			nc.rateLimit()
			n, err := nc.pullComments(ctx, store, pageID, rec.TaskID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: comments: %v", rec.TaskID, err))
			}
			result.Comments += n
		}
	}

	return result, nil
}

// pullUpdate applies Notion page changes to an existing Slate task.
func (nc *NotionClient) pullUpdate(ctx context.Context, store *Store, page *notionapi.Page, rec *NotionSyncRecord) error {
	updates := nc.propertiesToUpdate(page)

	// Pull description from page body blocks (if description is mapped to page body).
	if nc.Config.PropertyMap.Description == "" {
		nc.rateLimit()
		desc, err := nc.readPageBody(ctx, string(page.ID))
		if err == nil && desc != "" {
			updates.params.Description = &desc
			updates.fields = append(updates.fields, "description")
		}
	}

	if len(updates.fields) > 0 {
		_, err := store.Update(ctx, rec.TaskID, updates.params, "notion-sync")
		if err != nil {
			return fmt.Errorf("update task: %w", err)
		}
	}

	// Status is updated separately via UpdateStatus.
	if updates.status != nil {
		store.UpdateStatus(ctx, rec.TaskID, *updates.status, "notion-sync")
	}

	// Handle parent_id change.
	if updates.parentPageID != "" {
		parentRec, _ := store.GetSyncRecordByPage(ctx, updates.parentPageID)
		if parentRec != nil {
			parentID := parentRec.TaskID
			store.Update(ctx, rec.TaskID, UpdateParams{ParentID: &parentID}, "notion-sync")
		} else {
			// Parent not synced — try to pull-create it first.
			nc.rateLimit()
			parentPage, err := nc.API.GetPage(ctx, notionapi.PageID(updates.parentPageID))
			if err == nil && parentPage != nil {
				nc.pullCreate(ctx, store, parentPage)
				// Retry parent lookup.
				parentRec, _ = store.GetSyncRecordByPage(ctx, updates.parentPageID)
				if parentRec != nil {
					parentID := parentRec.TaskID
					store.Update(ctx, rec.TaskID, UpdateParams{ParentID: &parentID}, "notion-sync")
				}
			}
		}
	} else if updates.parentCleared {
		empty := ""
		store.Update(ctx, rec.TaskID, UpdateParams{ParentID: &empty}, "notion-sync")
	}

	// Pull dependencies.
	nc.pullDependencies(ctx, store, page, rec.TaskID)

	// Update sync timestamp.
	rec.LastSyncedAt = timeNowUTC()
	return store.UpsertSyncRecord(ctx, rec)
}

// pullCreate creates a new Slate task from a Notion page.
func (nc *NotionClient) pullCreate(ctx context.Context, store *Store, page *notionapi.Page) error {
	updates := nc.propertiesToUpdate(page)

	params := CreateParams{
		Title: updates.title,
	}
	if updates.params.Priority != nil {
		params.Priority = *updates.params.Priority
	}
	if updates.params.Assignee != nil {
		params.Assignee = *updates.params.Assignee
	}
	if updates.params.Labels != nil {
		params.Labels = *updates.params.Labels
	}
	if updates.params.Description != nil {
		params.Description = *updates.params.Description
	}

	// Pull description from page body blocks.
	if nc.Config.PropertyMap.Description == "" && params.Description == "" {
		nc.rateLimit()
		desc, err := nc.readPageBody(ctx, string(page.ID))
		if err == nil && desc != "" {
			params.Description = desc
		}
	}

	// Resolve parent from Notion relation.
	if updates.parentPageID != "" {
		parentRec, _ := store.GetSyncRecordByPage(ctx, updates.parentPageID)
		if parentRec != nil {
			params.ParentID = parentRec.TaskID
		}
	}

	task, err := store.Create(ctx, params)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Set status if not default (create always makes "open").
	if updates.status != nil && *updates.status != StatusOpen {
		store.UpdateStatus(ctx, task.ID, *updates.status, "notion-sync")
	}

	// Record sync mapping.
	return store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  string(page.ID),
		LastSyncedAt:  timeNowUTC(),
		SyncDirection: "both",
	})
}

// pullResult holds parsed fields from a Notion page.
type pullResult struct {
	title         string
	status        *Status   // separate from UpdateParams (uses UpdateStatus)
	params        UpdateParams
	fields        []string // which fields were set
	parentPageID  string   // Notion page ID of parent (from relation)
	parentCleared bool     // true if parent relation was cleared
}

// propertiesToUpdate converts Notion page properties to Slate UpdateParams.
func (nc *NotionClient) propertiesToUpdate(page *notionapi.Page) pullResult {
	result := pullResult{}
	cfg := nc.Config.PropertyMap

	// Title.
	if cfg.Title != "" {
		if prop, ok := page.Properties[cfg.Title]; ok {
			if tp, ok := prop.(*notionapi.TitleProperty); ok && len(tp.Title) > 0 {
				raw := tp.Title[0].PlainText
				// Strip [st-xxxx] prefix if present.
				result.title = stripSlatePrefix(raw)
				title := result.title
				result.params.Title = &title
				result.fields = append(result.fields, "title")
			}
		}
	}

	// Status.
	if cfg.Status != "" {
		if prop, ok := page.Properties[cfg.Status]; ok {
			if sp, ok := prop.(*notionapi.StatusProperty); ok && sp.Status.Name != "" {
				slateStatus := nc.Config.StatusFromNotion(sp.Status.Name)
				if slateStatus != "" {
					s := Status(slateStatus)
					result.status = &s
					result.fields = append(result.fields, "status")
				}
			}
		}
	}

	// Priority.
	if cfg.Priority != "" {
		if prop, ok := page.Properties[cfg.Priority]; ok {
			if sp, ok := prop.(*notionapi.SelectProperty); ok && sp.Select.Name != "" {
				p := Priority(nc.Config.PriorityFromNotion(sp.Select.Name))
				result.params.Priority = &p
				result.fields = append(result.fields, "priority")
			}
		}
	}

	// Assignee (people → first person's name).
	if cfg.Assignee != "" {
		if prop, ok := page.Properties[cfg.Assignee]; ok {
			if pp, ok := prop.(*notionapi.PeopleProperty); ok {
				if len(pp.People) > 0 {
					name := pp.People[0].Name
					result.params.Assignee = &name
					result.fields = append(result.fields, "assignee")
				} else {
					empty := ""
					result.params.Assignee = &empty
					result.fields = append(result.fields, "assignee")
				}
			}
		}
	}

	// Labels (multi-select).
	if cfg.Labels != "" {
		if prop, ok := page.Properties[cfg.Labels]; ok {
			if mp, ok := prop.(*notionapi.MultiSelectProperty); ok {
				var labels []string
				for _, opt := range mp.MultiSelect {
					labels = append(labels, opt.Name)
				}
				result.params.Labels = &labels
				result.fields = append(result.fields, "labels")
			}
		}
	}

	// Due date.
	if cfg.DueAt != "" {
		if prop, ok := page.Properties[cfg.DueAt]; ok {
			if dp, ok := prop.(*notionapi.DateProperty); ok {
				if dp.Date != nil && dp.Date.Start != nil {
					t := time.Time(*dp.Date.Start)
					result.params.DueAt = &t
					result.fields = append(result.fields, "due_at")
				}
			}
		}
	}

	// Parent relation (bidirectional).
	if cfg.ParentID != "" {
		if prop, ok := page.Properties[cfg.ParentID]; ok {
			if rp, ok := prop.(*notionapi.RelationProperty); ok {
				if len(rp.Relation) > 0 {
					result.parentPageID = string(rp.Relation[0].ID)
				} else {
					result.parentCleared = true
				}
			}
		}
	}

	return result
}

// pullDependencies syncs Notion relation properties back to Slate dependencies.
func (nc *NotionClient) pullDependencies(ctx context.Context, store *Store, page *notionapi.Page, taskID string) {
	for depType, propName := range nc.Config.DepMap {
		prop, ok := page.Properties[propName]
		if !ok {
			continue
		}
		rp, ok := prop.(*notionapi.RelationProperty)
		if !ok {
			continue
		}

		// Build set of current Notion relations.
		notionBlockers := make(map[string]bool)
		for _, rel := range rp.Relation {
			pageID := string(rel.ID)
			rec, _ := store.GetSyncRecordByPage(ctx, pageID)
			if rec != nil {
				notionBlockers[rec.TaskID] = true
				// Add dependency if not exists.
				store.AddDependency(ctx, rec.TaskID, taskID, DepType(depType))
			}
		}

		// Remove Slate deps that no longer exist in Notion.
		existingDeps, _ := store.ListDependents(ctx, taskID)
		for _, dep := range existingDeps {
			if string(dep.Type) == depType && !notionBlockers[dep.FromID] {
				store.RemoveDependency(ctx, dep.FromID, taskID)
			}
		}
	}
}

// pullComments syncs Notion page comments to Slate comments.
// Returns the number of new comments created.
func (nc *NotionClient) pullComments(ctx context.Context, store *Store, pageID, taskID string) (int, error) {
	// Get existing Slate comments to avoid duplicates.
	existingComments, err := store.ListComments(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("list comments: %w", err)
	}

	existingSet := make(map[string]bool)
	for _, c := range existingComments {
		if c.Author == "notion-sync" {
			existingSet[c.Content] = true
		}
	}

	var created int
	var cursor notionapi.Cursor
	for {
		var pagination *notionapi.Pagination
		if cursor != "" {
			pagination = &notionapi.Pagination{StartCursor: cursor}
		}

		resp, err := nc.API.GetPageComments(ctx, notionapi.BlockID(pageID), pagination)
		if err != nil {
			return created, fmt.Errorf("get comments: %w", err)
		}

		for _, comment := range resp.Results {
			// Extract plain text from rich text array.
			var text string
			for _, rt := range comment.RichText {
				text += rt.PlainText
			}
			if text == "" {
				continue
			}

			// Dedup check by content.
			if existingSet[text] {
				continue
			}

			_, err := store.AddComment(ctx, taskID, "notion-sync", text)
			if err != nil {
				continue
			}
			existingSet[text] = true
			created++
		}

		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
		nc.rateLimit()
	}

	return created, nil
}

// readPageBody reads a Notion page's body blocks and concatenates text content.
// Only reads paragraph, heading, and list item blocks — skips complex types.
func (nc *NotionClient) readPageBody(ctx context.Context, pageID string) (string, error) {
	var parts []string
	var cursor notionapi.Cursor
	for {
		var pagination *notionapi.Pagination
		if cursor != "" {
			pagination = &notionapi.Pagination{StartCursor: cursor}
		}

		resp, err := nc.API.GetBlockChildren(ctx, notionapi.BlockID(pageID), pagination)
		if err != nil {
			return "", fmt.Errorf("get block children: %w", err)
		}

		for _, block := range resp.Results {
			text := extractBlockText(block)
			if text != "" {
				parts = append(parts, text)
			}
		}

		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
		nc.rateLimit()
	}

	return strings.Join(parts, "\n"), nil
}

// extractBlockText extracts plain text from a Notion block.
func extractBlockText(block notionapi.Block) string {
	switch b := block.(type) {
	case *notionapi.ParagraphBlock:
		return richTextToPlain(b.Paragraph.RichText)
	case *notionapi.Heading1Block:
		return richTextToPlain(b.Heading1.RichText)
	case *notionapi.Heading2Block:
		return richTextToPlain(b.Heading2.RichText)
	case *notionapi.Heading3Block:
		return richTextToPlain(b.Heading3.RichText)
	case *notionapi.BulletedListItemBlock:
		return "- " + richTextToPlain(b.BulletedListItem.RichText)
	case *notionapi.NumberedListItemBlock:
		return "- " + richTextToPlain(b.NumberedListItem.RichText)
	default:
		return ""
	}
}

// richTextToPlain concatenates plain text from a RichText array.
func richTextToPlain(rts []notionapi.RichText) string {
	var s string
	for _, rt := range rts {
		s += rt.PlainText
	}
	return s
}

// stripSlatePrefix removes "[st-xxxx] " prefix from a title string.
func stripSlatePrefix(title string) string {
	if len(title) > 2 && title[0] == '[' {
		end := strings.Index(title, "] ")
		if end > 0 {
			return title[end+2:]
		}
	}
	return title
}
