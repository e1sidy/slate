package slate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jomei/notionapi"
)

// SyncResult summarizes the outcome of a bidirectional sync.
type SyncResult struct {
	Pushed    PushResult
	Pulled    PullResult
	Conflicts []ConflictInfo
}

// ConflictInfo describes a field-level conflict between Slate and Notion.
type ConflictInfo struct {
	TaskID          string    `json:"task_id"`
	Field           string    `json:"field"`
	SlateValue      string    `json:"slate_value"`
	NotionValue     string    `json:"notion_value"`
	SlateUpdatedAt  time.Time `json:"slate_updated_at"`
	NotionUpdatedAt time.Time `json:"notion_updated_at"`
	Resolution      string    `json:"resolution"` // "last-write-wins", "manual:local", "manual:notion"
}

// Sync performs bidirectional sync: push local changes, pull remote changes,
// detect and resolve conflicts.
func (nc *NotionClient) Sync(ctx context.Context, store *Store, filter ListParams) (*SyncResult, error) {
	result := &SyncResult{}

	// Step 1: Detect conflicts on already-synced tasks.
	records, err := store.ListSyncRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sync records: %w", err)
	}

	for _, rec := range records {
		nc.rateLimit()
		page, err := nc.API.GetPage(ctx, notionapi.PageID(rec.NotionPageID))
		if err != nil {
			continue // page may be deleted
		}

		task, err := store.Get(ctx, rec.TaskID)
		if err != nil {
			continue // task may be deleted
		}

		conflicts := nc.detectConflicts(ctx, store, task, page, rec)
		for i := range conflicts {
			// Auto-resolve with last-write-wins.
			if conflicts[i].SlateUpdatedAt.After(conflicts[i].NotionUpdatedAt) {
				conflicts[i].Resolution = "last-write-wins:local"
			} else {
				conflicts[i].Resolution = "last-write-wins:notion"
			}
		}
		result.Conflicts = append(result.Conflicts, conflicts...)

		// Record unresolved conflicts in sync table.
		if len(conflicts) > 0 {
			conflictJSON, _ := json.Marshal(conflicts)
			rec.ConflictStatus = string(conflictJSON)
			store.UpsertSyncRecord(ctx, rec)
		}
	}

	// Step 2: Push local changes.
	pushResult, err := nc.PushAll(ctx, store, filter)
	if err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}
	result.Pushed = *pushResult

	// Step 3: Pull remote changes.
	pullResult, err := nc.PullChanges(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("pull: %w", err)
	}
	result.Pulled = *pullResult

	return result, nil
}

// detectConflicts checks if both Slate and Notion have changed the same field
// since the last sync. Returns a list of conflicting fields.
func (nc *NotionClient) detectConflicts(ctx context.Context, store *Store, task *Task, page *notionapi.Page, rec *NotionSyncRecord) []ConflictInfo {
	var conflicts []ConflictInfo
	syncTime := rec.LastSyncedAt
	notionEditTime := page.LastEditedTime

	// Only check if Notion was edited after last sync.
	if !notionEditTime.After(syncTime) {
		return nil
	}

	cfg := nc.Config.PropertyMap

	// Check status.
	if cfg.Status != "" {
		slateEvents, _ := store.EventsSince(ctx, task.ID, syncTime)
		slateChanged, slateTime := fieldChangedSince(slateEvents, "status")
		notionStatus := extractNotionStatus(page, cfg.Status)

		if slateChanged && notionStatus != "" {
			localStatus := string(task.Status)
			notionMapped := nc.Config.StatusFromNotion(notionStatus)
			if notionMapped != "" && notionMapped != localStatus {
				conflicts = append(conflicts, ConflictInfo{
					TaskID:          task.ID,
					Field:           "status",
					SlateValue:      localStatus,
					NotionValue:     notionStatus,
					SlateUpdatedAt:  slateTime,
					NotionUpdatedAt: notionEditTime,
				})
			}
		}
	}

	// Check priority.
	if cfg.Priority != "" {
		slateEvents, _ := store.EventsSince(ctx, task.ID, syncTime)
		slateChanged, slateTime := fieldChangedSince(slateEvents, "priority")
		notionPriority := extractNotionSelect(page, cfg.Priority)

		if slateChanged && notionPriority != "" {
			localPriority := nc.Config.PriorityToNotion(int(task.Priority))
			if notionPriority != localPriority {
				conflicts = append(conflicts, ConflictInfo{
					TaskID:          task.ID,
					Field:           "priority",
					SlateValue:      localPriority,
					NotionValue:     notionPriority,
					SlateUpdatedAt:  slateTime,
					NotionUpdatedAt: notionEditTime,
				})
			}
		}
	}

	// Check assignee.
	if cfg.Assignee != "" {
		slateEvents, _ := store.EventsSince(ctx, task.ID, syncTime)
		slateChanged, slateTime := fieldChangedSince(slateEvents, "assignee")
		notionAssignee := extractNotionPeople(page, cfg.Assignee)

		if slateChanged && notionAssignee != task.Assignee {
			conflicts = append(conflicts, ConflictInfo{
				TaskID:          task.ID,
				Field:           "assignee",
				SlateValue:      task.Assignee,
				NotionValue:     notionAssignee,
				SlateUpdatedAt:  slateTime,
				NotionUpdatedAt: notionEditTime,
			})
		}
	}

	return conflicts
}

// ResolveConflict manually resolves all conflicts for a task.
// preference is "local" or "notion".
func (nc *NotionClient) ResolveConflict(ctx context.Context, store *Store, taskID, preference string) error {
	rec, err := store.GetSyncRecord(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get sync record: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("task %s is not synced", taskID)
	}
	if rec.ConflictStatus == "" {
		return fmt.Errorf("task %s has no conflicts", taskID)
	}

	switch preference {
	case "local":
		// Push local state to Notion.
		task, err := store.GetFull(ctx, taskID)
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}
		if err := nc.pushUpdate(ctx, store, task, rec.NotionPageID); err != nil {
			return fmt.Errorf("push local: %w", err)
		}
	case "notion":
		// Pull Notion state to local.
		nc.rateLimit()
		page, err := nc.API.GetPage(ctx, notionapi.PageID(rec.NotionPageID))
		if err != nil {
			return fmt.Errorf("get notion page: %w", err)
		}
		if err := nc.pullUpdate(ctx, store, page, rec); err != nil {
			return fmt.Errorf("pull notion: %w", err)
		}
	default:
		return fmt.Errorf("preference must be 'local' or 'notion', got %q", preference)
	}

	// Clear conflict status.
	rec.ConflictStatus = ""
	rec.LastSyncedAt = timeNowUTC()
	return store.UpsertSyncRecord(ctx, rec)
}

// --- Helpers ---

// fieldChangedSince checks if a specific field was changed in events after the given time.
func fieldChangedSince(events []*Event, field string) (bool, time.Time) {
	var latest time.Time
	changed := false
	for _, e := range events {
		if e.Field == field {
			changed = true
			if e.Timestamp.After(latest) {
				latest = e.Timestamp
			}
		}
	}
	return changed, latest
}

// extractNotionStatus gets the status option name from a Notion page.
func extractNotionStatus(page *notionapi.Page, propName string) string {
	prop, ok := page.Properties[propName]
	if !ok {
		return ""
	}
	sp, ok := prop.(*notionapi.StatusProperty)
	if !ok {
		return ""
	}
	return sp.Status.Name
}

// extractNotionSelect gets a select option name from a Notion page.
func extractNotionSelect(page *notionapi.Page, propName string) string {
	prop, ok := page.Properties[propName]
	if !ok {
		return ""
	}
	sp, ok := prop.(*notionapi.SelectProperty)
	if !ok {
		return ""
	}
	return sp.Select.Name
}

// extractNotionPeople gets the first person's name from a Notion page.
func extractNotionPeople(page *notionapi.Page, propName string) string {
	prop, ok := page.Properties[propName]
	if !ok {
		return ""
	}
	pp, ok := prop.(*notionapi.PeopleProperty)
	if !ok || len(pp.People) == 0 {
		return ""
	}
	return pp.People[0].Name
}
