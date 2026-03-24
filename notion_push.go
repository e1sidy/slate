package slate

import (
	"context"
	"fmt"
	"sort"

	"github.com/jomei/notionapi"
)

// PushResult summarizes the outcome of a push sync operation.
type PushResult struct {
	Created  int
	Updated  int
	Skipped  int
	Errors   []string
}

// PushTask pushes a single Slate task to Notion (create or update).
func (nc *NotionClient) PushTask(ctx context.Context, store *Store, taskID string) error {
	task, err := store.GetFull(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task %s: %w", taskID, err)
	}

	rec, err := store.GetSyncRecord(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get sync record: %w", err)
	}

	if rec != nil {
		return nc.pushUpdate(ctx, store, task, rec.NotionPageID)
	}
	return nc.pushCreate(ctx, store, task)
}

// PushAll pushes all matching Slate tasks to Notion.
// Uses two-pass strategy: create/update all pages, then set parent relations.
func (nc *NotionClient) PushAll(ctx context.Context, store *Store, filter ListParams) (*PushResult, error) {
	tasks, err := store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Sort by depth: roots first, then children.
	sortByDepth(tasks)

	result := &PushResult{}

	// Pass 1: Create/update all pages (without parent relations for new pages).
	for _, task := range tasks {
		full, err := store.GetFull(ctx, task.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: get full: %v", task.ID, err))
			result.Skipped++
			continue
		}

		rec, err := store.GetSyncRecord(ctx, task.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: get sync: %v", task.ID, err))
			result.Skipped++
			continue
		}

		nc.rateLimit()

		if rec != nil {
			if err := nc.pushUpdate(ctx, store, full, rec.NotionPageID); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: update: %v", task.ID, err))
				result.Skipped++
				continue
			}
			result.Updated++
		} else {
			if err := nc.pushCreate(ctx, store, full); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: create: %v", task.ID, err))
				result.Skipped++
				continue
			}
			result.Created++
		}
	}

	// Pass 2: Set parent relations (now all pages exist).
	for _, task := range tasks {
		if task.ParentID == "" {
			continue
		}
		rec, err := store.GetSyncRecord(ctx, task.ID)
		if err != nil || rec == nil {
			continue
		}
		parentRec, err := store.GetSyncRecord(ctx, task.ParentID)
		if err != nil || parentRec == nil {
			continue // parent not synced
		}

		propName := nc.Config.PropertyMap.ParentID
		if propName == "" {
			continue
		}

		nc.rateLimit()
		_, err = nc.API.UpdatePage(ctx, notionapi.PageID(rec.NotionPageID), &notionapi.PageUpdateRequest{
			Properties: notionapi.Properties{
				propName: notionapi.RelationProperty{
					Type:     notionapi.PropertyTypeRelation,
					Relation: []notionapi.Relation{{ID: notionapi.PageID(parentRec.NotionPageID)}},
				},
			},
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: set parent relation: %v", task.ID, err))
		}
	}

	// Pass 3: Set dependency relations.
	for _, task := range tasks {
		if err := nc.pushDependencies(ctx, store, task.ID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: deps: %v", task.ID, err))
		}
	}

	return result, nil
}

// pushCreate creates a new Notion page for a Slate task.
func (nc *NotionClient) pushCreate(ctx context.Context, store *Store, task *Task) error {
	props := nc.taskToProperties(task)

	// Add PR links from custom attribute.
	nc.addPRLinks(ctx, store, task.ID, props)

	// Add latest checkpoint as progress.
	if progressProp := nc.Config.PropertyMap.Progress; progressProp != "" {
		cp, _ := store.LatestCheckpoint(ctx, task.ID)
		if cp != nil && cp.Done != "" {
			props[progressProp] = notionapi.RichTextProperty{
				Type:     notionapi.PropertyTypeRichText,
				RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: cp.Done}}},
			}
		}
	}

	req := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(nc.Config.DatabaseID),
		},
		Properties: props,
	}

	// Add description as page body if configured.
	if nc.Config.PropertyMap.Description == "" && task.Description != "" {
		req.Children = descriptionToBlocks(task.Description)
	}

	page, err := nc.API.CreatePage(ctx, req)
	if err != nil {
		return fmt.Errorf("create notion page: %w", err)
	}

	// Record the sync.
	return store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  string(page.ID),
		LastSyncedAt:  timeNowUTC(),
		SyncDirection: "both",
	})
}

// pushUpdate updates an existing Notion page from a Slate task.
func (nc *NotionClient) pushUpdate(ctx context.Context, store *Store, task *Task, pageID string) error {
	props := nc.taskToProperties(task)

	// Add PR links from custom attribute.
	nc.addPRLinks(ctx, store, task.ID, props)

	// Add latest checkpoint as progress.
	if progressProp := nc.Config.PropertyMap.Progress; progressProp != "" {
		cp, _ := store.LatestCheckpoint(ctx, task.ID)
		if cp != nil && cp.Done != "" {
			props[progressProp] = notionapi.RichTextProperty{
				Type:     notionapi.PropertyTypeRichText,
				RichText: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: cp.Done}}},
			}
		}
	}

	// Set parent relation if parent is synced.
	if parentProp := nc.Config.PropertyMap.ParentID; parentProp != "" && task.ParentID != "" {
		parentRec, _ := store.GetSyncRecord(ctx, task.ParentID)
		if parentRec != nil {
			props[parentProp] = notionapi.RelationProperty{
				Type:     notionapi.PropertyTypeRelation,
				Relation: []notionapi.Relation{{ID: notionapi.PageID(parentRec.NotionPageID)}},
			}
		}
	}

	_, err := nc.API.UpdatePage(ctx, notionapi.PageID(pageID), &notionapi.PageUpdateRequest{
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("update notion page: %w", err)
	}

	// Update sync timestamp.
	return store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  pageID,
		LastSyncedAt:  timeNowUTC(),
		SyncDirection: "both",
	})
}

// taskToProperties converts a Slate task to Notion page properties.
func (nc *NotionClient) taskToProperties(task *Task) notionapi.Properties {
	props := notionapi.Properties{}
	cfg := nc.Config.PropertyMap

	// Title: "[st-ab12] Task title"
	if cfg.Title != "" {
		titleText := fmt.Sprintf("[%s] %s", task.ID, task.Title)
		props[cfg.Title] = notionapi.TitleProperty{
			Type:  notionapi.PropertyTypeTitle,
			Title: []notionapi.RichText{{Type: "text", Text: &notionapi.Text{Content: titleText}}},
		}
	}

	// Status.
	if cfg.Status != "" {
		notionStatus := nc.Config.StatusToNotion(string(task.Status))
		props[cfg.Status] = notionapi.StatusProperty{
			Type:   notionapi.PropertyTypeStatus,
			Status: notionapi.Status{Name: notionStatus},
		}
	}

	// Priority.
	if cfg.Priority != "" {
		notionPriority := nc.Config.PriorityToNotion(int(task.Priority))
		props[cfg.Priority] = notionapi.SelectProperty{
			Type:   notionapi.PropertyTypeSelect,
			Select: notionapi.Option{Name: notionPriority},
		}
	}

	// Assignee (people type).
	if cfg.Assignee != "" && task.Assignee != "" {
		if user, ok := nc.LookupUser(task.Assignee); ok {
			props[cfg.Assignee] = notionapi.PeopleProperty{
				Type:   notionapi.PropertyTypePeople,
				People: []notionapi.User{user},
			}
		}
	}

	// Labels (multi-select).
	if cfg.Labels != "" && len(task.Labels) > 0 {
		var options []notionapi.Option
		for _, label := range task.Labels {
			if label != "" {
				options = append(options, notionapi.Option{Name: label})
			}
		}
		if len(options) > 0 {
			props[cfg.Labels] = notionapi.MultiSelectProperty{
				Type:        notionapi.PropertyTypeMultiSelect,
				MultiSelect: options,
			}
		}
	}

	// Due date.
	if cfg.DueAt != "" && task.DueAt != nil {
		isoDate := notionapi.Date(*task.DueAt)
		props[cfg.DueAt] = notionapi.DateProperty{
			Type: notionapi.PropertyTypeDate,
			Date: &notionapi.DateObject{Start: &isoDate},
		}
	}

	// Type (select).
	if cfg.Type != "" {
		props[cfg.Type] = notionapi.SelectProperty{
			Type:   notionapi.PropertyTypeSelect,
			Select: notionapi.Option{Name: string(task.Type)},
		}
	}

	return props
}

// pushDependencies syncs Slate dependencies to Notion relation properties.
// For a given task, finds all tasks that block it (dependents) and sets the
// corresponding Notion relation properties (e.g., "Blocked by").
func (nc *NotionClient) pushDependencies(ctx context.Context, store *Store, taskID string) error {
	rec, err := store.GetSyncRecord(ctx, taskID)
	if err != nil || rec == nil {
		return nil // not synced, skip
	}

	// ListDependents returns deps where to_id = taskID (tasks that block this task).
	deps, err := store.ListDependents(ctx, taskID)
	if err != nil {
		return fmt.Errorf("list dependents: %w", err)
	}

	// Group deps by type. Each dep has from_id (the blocker) blocking to_id (this task).
	depsByType := make(map[string][]notionapi.Relation)
	for _, dep := range deps {
		otherRec, err := store.GetSyncRecord(ctx, dep.FromID)
		if err != nil || otherRec == nil {
			continue // blocker not synced
		}

		notionProp, ok := nc.Config.DepMap[string(dep.Type)]
		if !ok {
			continue // no mapping for this dep type
		}

		depsByType[notionProp] = append(depsByType[notionProp],
			notionapi.Relation{ID: notionapi.PageID(otherRec.NotionPageID)})
	}

	if len(depsByType) == 0 {
		return nil
	}

	props := notionapi.Properties{}
	for propName, relations := range depsByType {
		props[propName] = notionapi.RelationProperty{
			Type:     notionapi.PropertyTypeRelation,
			Relation: relations,
		}
	}

	nc.rateLimit()
	_, err = nc.API.UpdatePage(ctx, notionapi.PageID(rec.NotionPageID), &notionapi.PageUpdateRequest{
		Properties: props,
	})
	return err
}

// addPRLinks reads the pr_url custom attribute and adds it as a Notion URL property.
func (nc *NotionClient) addPRLinks(ctx context.Context, store *Store, taskID string, props notionapi.Properties) {
	prProp := nc.Config.PropertyMap.PRLinks
	if prProp == "" {
		return
	}
	attr, err := store.GetAttr(ctx, taskID, "pr_url")
	if err != nil || attr == nil || attr.Value == "" {
		return
	}
	props[prProp] = notionapi.URLProperty{
		Type: notionapi.PropertyTypeURL,
		URL:  attr.Value,
	}
}

// descriptionToBlocks converts a description string to Notion paragraph blocks.
func descriptionToBlocks(desc string) []notionapi.Block {
	if desc == "" {
		return nil
	}
	return []notionapi.Block{
		notionapi.ParagraphBlock{
			BasicBlock: notionapi.BasicBlock{
				Object: notionapi.ObjectTypeBlock,
				Type:   notionapi.BlockTypeParagraph,
			},
			Paragraph: notionapi.Paragraph{
				RichText: []notionapi.RichText{
					{Type: "text", Text: &notionapi.Text{Content: desc}},
				},
			},
		},
	}
}

// sortByDepth sorts tasks so parents come before children.
func sortByDepth(tasks []*Task) {
	// Build depth map.
	idMap := make(map[string]*Task)
	for _, t := range tasks {
		idMap[t.ID] = t
	}

	depth := make(map[string]int)
	var getDepth func(id string) int
	getDepth = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		t, ok := idMap[id]
		if !ok || t.ParentID == "" {
			depth[id] = 0
			return 0
		}
		d := getDepth(t.ParentID) + 1
		depth[id] = d
		return d
	}

	for _, t := range tasks {
		getDepth(t.ID)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return depth[tasks[i].ID] < depth[tasks[j].ID]
	})
}
