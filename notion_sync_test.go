package slate

import (
	"context"
	"testing"
	"time"

	"github.com/jomei/notionapi"
)

func TestDetectConflicts_NoConflict(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	past := time.Now().Add(-time.Hour)
	rec := &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  past,
		SyncDirection: "both",
	}

	// Notion page not modified since sync.
	page := &notionapi.Page{
		ID:             notionapi.ObjectID("page-1"),
		LastEditedTime: past.Add(-time.Minute), // before sync
		Properties:     notionapi.Properties{},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	conflicts := client.detectConflicts(ctx, store, task, page, rec)
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestDetectConflicts_StatusConflict(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	// Set sync time in the past (before we make local changes).
	past := timeNowUTC().Add(-time.Hour)
	rec := &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  past,
		SyncDirection: "both",
	}

	// Change status locally (creates an event after sync time).
	store.UpdateStatus(ctx, task.ID, StatusInProgress, "user")
	// Re-fetch task to get updated status.
	task, _ = store.Get(ctx, task.ID)

	// Notion also changed status (to something different from local).
	page := &notionapi.Page{
		ID:             notionapi.ObjectID("page-1"),
		LastEditedTime: time.Now().Add(time.Second), // after sync
		Properties: notionapi.Properties{
			"Status": &notionapi.StatusProperty{
				Type:   notionapi.PropertyTypeStatus,
				Status: notionapi.Status{Name: "Blocked"},
			},
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	conflicts := client.detectConflicts(ctx, store, task, page, rec)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].Field != "status" {
		t.Errorf("field = %q, want status", conflicts[0].Field)
	}
	if conflicts[0].NotionValue != "Blocked" {
		t.Errorf("notion_value = %q, want Blocked", conflicts[0].NotionValue)
	}
}

func TestDetectConflicts_PriorityConflict(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test", Priority: P2})
	past := timeNowUTC().Add(-time.Hour)
	rec := &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  past,
		SyncDirection: "both",
	}

	// Change priority locally (creates event after sync time).
	p := Priority(P1)
	store.Update(ctx, task.ID, UpdateParams{Priority: &p}, "user")
	task, _ = store.Get(ctx, task.ID)

	// Notion also changed priority.
	page := &notionapi.Page{
		ID:             notionapi.ObjectID("page-1"),
		LastEditedTime: time.Now().Add(time.Second),
		Properties: notionapi.Properties{
			"Priority": &notionapi.SelectProperty{
				Type:   notionapi.PropertyTypeSelect,
				Select: notionapi.Option{Name: "Low"},
			},
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	conflicts := client.detectConflicts(ctx, store, task, page, rec)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].Field != "priority" {
		t.Errorf("field = %q, want priority", conflicts[0].Field)
	}
}

func TestResolveConflict_PreferLocal(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:         task.ID,
		NotionPageID:   "page-1",
		LastSyncedAt:   time.Now().Add(-time.Hour),
		SyncDirection:  "both",
		ConflictStatus: `[{"field":"status"}]`,
	})

	updateCalled := false
	mock := &mockNotionAPI{
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			updateCalled = true
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	if err := client.ResolveConflict(ctx, store, task.ID, "local"); err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}

	if !updateCalled {
		t.Error("should push local to Notion")
	}

	// Conflict should be cleared.
	rec, _ := store.GetSyncRecord(ctx, task.ID)
	if rec.ConflictStatus != "" {
		t.Errorf("conflict_status = %q, want empty", rec.ConflictStatus)
	}
}

func TestResolveConflict_PreferNotion(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:         task.ID,
		NotionPageID:   "page-1",
		LastSyncedAt:   time.Now().Add(-time.Hour),
		SyncDirection:  "both",
		ConflictStatus: `[{"field":"priority"}]`,
	})

	mock := &mockNotionAPI{
		getPageFn: func(ctx context.Context, id notionapi.PageID) (*notionapi.Page, error) {
			return &notionapi.Page{
				ID:             notionapi.ObjectID(id),
				LastEditedTime: time.Now(),
				Properties: notionapi.Properties{
					"Task name": &notionapi.TitleProperty{
						Type:  notionapi.PropertyTypeTitle,
						Title: []notionapi.RichText{{PlainText: "Updated", Text: &notionapi.Text{Content: "Updated"}}},
					},
					"Priority": &notionapi.SelectProperty{
						Type:   notionapi.PropertyTypeSelect,
						Select: notionapi.Option{Name: "Low"},
					},
				},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	if err := client.ResolveConflict(ctx, store, task.ID, "notion"); err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}

	// Conflict should be cleared.
	rec, _ := store.GetSyncRecord(ctx, task.ID)
	if rec.ConflictStatus != "" {
		t.Errorf("conflict_status = %q, want empty", rec.ConflictStatus)
	}

	// Task should be updated from Notion.
	updated, _ := store.Get(ctx, task.ID)
	if updated.Priority != P3 {
		t.Errorf("priority = %d, want %d (Low → P3)", updated.Priority, P3)
	}
}

func TestResolveConflict_InvalidPreference(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:         task.ID,
		NotionPageID:   "page-1",
		LastSyncedAt:   time.Now(),
		SyncDirection:  "both",
		ConflictStatus: "conflict",
	})

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	err := client.ResolveConflict(ctx, store, task.ID, "invalid")
	if err == nil {
		t.Error("should error on invalid preference")
	}
}

func TestResolveConflict_NoConflicts(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  time.Now(),
		SyncDirection: "both",
	})

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	err := client.ResolveConflict(ctx, store, task.ID, "local")
	if err == nil {
		t.Error("should error when no conflicts exist")
	}
}

func TestFieldChangedSince(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	now := time.Now()

	events := []*Event{
		{Field: "status", Timestamp: now},
		{Field: "title", Timestamp: past},
	}

	changed, ts := fieldChangedSince(events, "status")
	if !changed {
		t.Error("status should be changed")
	}
	if ts.IsZero() {
		t.Error("timestamp should not be zero")
	}

	changed2, _ := fieldChangedSince(events, "priority")
	if changed2 {
		t.Error("priority should not be changed")
	}
}

func TestExtractNotionHelpers(t *testing.T) {
	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Status": &notionapi.StatusProperty{
				Type:   notionapi.PropertyTypeStatus,
				Status: notionapi.Status{Name: "Done"},
			},
			"Priority": &notionapi.SelectProperty{
				Type:   notionapi.PropertyTypeSelect,
				Select: notionapi.Option{Name: "High"},
			},
			"Assignee": &notionapi.PeopleProperty{
				Type:   notionapi.PropertyTypePeople,
				People: []notionapi.User{{Name: "Alice"}},
			},
		},
	}

	if got := extractNotionStatus(page, "Status"); got != "Done" {
		t.Errorf("status = %q, want Done", got)
	}
	if got := extractNotionSelect(page, "Priority"); got != "High" {
		t.Errorf("priority = %q, want High", got)
	}
	if got := extractNotionPeople(page, "Assignee"); got != "Alice" {
		t.Errorf("assignee = %q, want Alice", got)
	}

	// Missing properties.
	if got := extractNotionStatus(page, "Missing"); got != "" {
		t.Errorf("missing status = %q, want empty", got)
	}
	if got := extractNotionPeople(page, "Missing"); got != "" {
		t.Errorf("missing people = %q, want empty", got)
	}
}
