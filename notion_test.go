package slate

import (
	"context"
	"testing"
	"time"

	"github.com/jomei/notionapi"
)

// --- Mock NotionAPI for testing ---

type mockNotionAPI struct {
	database      *notionapi.Database
	getDatabaseFn func(ctx context.Context, id notionapi.DatabaseID) (*notionapi.Database, error)
	queryFn       func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error)
	updateDBFn    func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseUpdateRequest) (*notionapi.Database, error)
	createPageFn  func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error)
	updatePageFn  func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error)
	getPageFn     func(ctx context.Context, id notionapi.PageID) (*notionapi.Page, error)
	commentsFn    func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error)
	createCmtFn   func(ctx context.Context, req *notionapi.CommentCreateRequest) (*notionapi.Comment, error)
	listUsersFn   func(ctx context.Context, pagination *notionapi.Pagination) (*notionapi.UsersListResponse, error)
	getChildrenFn func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error)
	appendChildFn func(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error)
}

func (m *mockNotionAPI) GetDatabase(ctx context.Context, id notionapi.DatabaseID) (*notionapi.Database, error) {
	if m.getDatabaseFn != nil {
		return m.getDatabaseFn(ctx, id)
	}
	return m.database, nil
}

func (m *mockNotionAPI) QueryDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, id, req)
	}
	return &notionapi.DatabaseQueryResponse{}, nil
}

func (m *mockNotionAPI) UpdateDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseUpdateRequest) (*notionapi.Database, error) {
	if m.updateDBFn != nil {
		return m.updateDBFn(ctx, id, req)
	}
	return m.database, nil
}

func (m *mockNotionAPI) CreatePage(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
	if m.createPageFn != nil {
		return m.createPageFn(ctx, req)
	}
	return &notionapi.Page{ID: notionapi.ObjectID("page-1")}, nil
}

func (m *mockNotionAPI) UpdatePage(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
	if m.updatePageFn != nil {
		return m.updatePageFn(ctx, id, req)
	}
	return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
}

func (m *mockNotionAPI) GetPage(ctx context.Context, id notionapi.PageID) (*notionapi.Page, error) {
	if m.getPageFn != nil {
		return m.getPageFn(ctx, id)
	}
	return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
}

func (m *mockNotionAPI) GetPageComments(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error) {
	if m.commentsFn != nil {
		return m.commentsFn(ctx, id, pagination)
	}
	return &notionapi.CommentQueryResponse{}, nil
}

func (m *mockNotionAPI) CreateComment(ctx context.Context, req *notionapi.CommentCreateRequest) (*notionapi.Comment, error) {
	if m.createCmtFn != nil {
		return m.createCmtFn(ctx, req)
	}
	return &notionapi.Comment{}, nil
}

func (m *mockNotionAPI) ListUsers(ctx context.Context, pagination *notionapi.Pagination) (*notionapi.UsersListResponse, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, pagination)
	}
	return &notionapi.UsersListResponse{
		Results: []notionapi.User{
			{ID: "user-1", Name: "Alice"},
			{ID: "user-2", Name: "Bob"},
		},
	}, nil
}

func (m *mockNotionAPI) GetBlockChildren(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error) {
	if m.getChildrenFn != nil {
		return m.getChildrenFn(ctx, id, pagination)
	}
	return &notionapi.GetChildrenResponse{}, nil
}

func (m *mockNotionAPI) AppendBlockChildren(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error) {
	if m.appendChildFn != nil {
		return m.appendChildFn(ctx, id, req)
	}
	return &notionapi.AppendBlockChildrenResponse{}, nil
}

// --- Tests ---

func TestNotionClient_Ping(t *testing.T) {
	mock := &mockNotionAPI{
		database: &notionapi.Database{ID: "db-1"},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0 // no delay in tests

	client := NewNotionClientWithAPI(mock, &cfg)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Users should be cached.
	if len(client.Users) != 2 {
		t.Errorf("users = %d, want 2", len(client.Users))
	}
	if u, ok := client.LookupUser("Alice"); !ok || string(u.ID) != "user-1" {
		t.Error("Alice not found or wrong ID")
	}
}

func TestNotionClient_PingFail(t *testing.T) {
	mock := &mockNotionAPI{
		getDatabaseFn: func(ctx context.Context, id notionapi.DatabaseID) (*notionapi.Database, error) {
			return nil, context.DeadlineExceeded
		},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "bad"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0

	client := NewNotionClientWithAPI(mock, &cfg)
	if err := client.Ping(context.Background()); err == nil {
		t.Error("Ping should fail with bad token")
	}
}

func TestNotionClient_EnsureProperties_AllExist(t *testing.T) {
	mock := &mockNotionAPI{
		database: &notionapi.Database{
			ID: "db-1",
			Properties: notionapi.PropertyConfigs{
				"Task name": &notionapi.TitlePropertyConfig{Type: notionapi.PropertyConfigTypeTitle},
				"Status":    &notionapi.StatusPropertyConfig{Type: notionapi.PropertyConfigStatus},
				"Priority":  &notionapi.SelectPropertyConfig{Type: notionapi.PropertyConfigTypeSelect},
				"Assignee":  &notionapi.PeoplePropertyConfig{Type: notionapi.PropertyConfigTypePeople},
				"Tags":      &notionapi.MultiSelectPropertyConfig{Type: notionapi.PropertyConfigTypeMultiSelect},
				"Due Date":  &notionapi.DatePropertyConfig{Type: notionapi.PropertyConfigTypeDate},
			},
		},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	// Only map properties that exist.
	cfg.PropertyMap.ParentID = ""
	cfg.PropertyMap.Type = ""
	cfg.PropertyMap.Progress = ""

	client := NewNotionClientWithAPI(mock, &cfg)
	created, warnings, err := client.EnsureProperties(context.Background())
	if err != nil {
		t.Fatalf("EnsureProperties: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("created = %v, want empty", created)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want empty", warnings)
	}
}

func TestNotionClient_EnsureProperties_TypeMismatch(t *testing.T) {
	mock := &mockNotionAPI{
		database: &notionapi.Database{
			ID: "db-1",
			Properties: notionapi.PropertyConfigs{
				"Task name": &notionapi.TitlePropertyConfig{Type: notionapi.PropertyConfigTypeTitle},
				"Status":    &notionapi.StatusPropertyConfig{Type: notionapi.PropertyConfigStatus},
				"Priority":  &notionapi.FilesPropertyConfig{Type: notionapi.PropertyConfigTypeFiles}, // wrong type!
				"Assignee":  &notionapi.PeoplePropertyConfig{Type: notionapi.PropertyConfigTypePeople},
				"Tags":      &notionapi.MultiSelectPropertyConfig{Type: notionapi.PropertyConfigTypeMultiSelect},
				"Due Date":  &notionapi.DatePropertyConfig{Type: notionapi.PropertyConfigTypeDate},
			},
		},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	cfg.PropertyMap.ParentID = ""
	cfg.PropertyMap.Type = ""
	cfg.PropertyMap.Progress = ""

	client := NewNotionClientWithAPI(mock, &cfg)
	_, warnings, err := client.EnsureProperties(context.Background())
	if err != nil {
		t.Fatalf("EnsureProperties: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(warnings))
	}
	if warnings[0] == "" {
		t.Error("warning should mention type mismatch")
	}
}

func TestNotionClient_EnsureProperties_AutoCreate(t *testing.T) {
	updateCalled := false
	mock := &mockNotionAPI{
		database: &notionapi.Database{
			ID: "db-1",
			Properties: notionapi.PropertyConfigs{
				"Task name": &notionapi.TitlePropertyConfig{Type: notionapi.PropertyConfigTypeTitle},
				"Status":    &notionapi.StatusPropertyConfig{Type: notionapi.PropertyConfigStatus},
				"Priority":  &notionapi.SelectPropertyConfig{Type: notionapi.PropertyConfigTypeSelect},
				"Assignee":  &notionapi.PeoplePropertyConfig{Type: notionapi.PropertyConfigTypePeople},
				"Tags":      &notionapi.MultiSelectPropertyConfig{Type: notionapi.PropertyConfigTypeMultiSelect},
				"Due Date":  &notionapi.DatePropertyConfig{Type: notionapi.PropertyConfigTypeDate},
			},
		},
		updateDBFn: func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseUpdateRequest) (*notionapi.Database, error) {
			updateCalled = true
			return &notionapi.Database{ID: notionapi.ObjectID(id)}, nil
		},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	cfg.PropertyMap.ParentID = "" // skip
	cfg.PropertyMap.Type = "Type" // auto-create
	cfg.PropertyMap.Progress = "Progress"
	cfg.AutoCreateProperties = true

	client := NewNotionClientWithAPI(mock, &cfg)
	created, _, err := client.EnsureProperties(context.Background())
	if err != nil {
		t.Fatalf("EnsureProperties: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateDatabase should have been called")
	}
	if len(created) != 2 {
		t.Errorf("created = %v, want [Type, Progress]", created)
	}
}

func TestNotionClient_EnsureProperties_NoAutoCreate(t *testing.T) {
	mock := &mockNotionAPI{
		database: &notionapi.Database{
			ID:         "db-1",
			Properties: notionapi.PropertyConfigs{},
		},
	}
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	cfg.AutoCreateProperties = false

	client := NewNotionClientWithAPI(mock, &cfg)
	created, warnings, err := client.EnsureProperties(context.Background())
	if err != nil {
		t.Fatalf("EnsureProperties: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("should not create when auto_create=false, got %v", created)
	}
	if len(warnings) == 0 {
		t.Error("should warn about missing properties")
	}
}

// --- Sync Record Tests ---

func TestSyncRecord_CRUD(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	// Initially empty.
	records, err := store.ListSyncRecords(ctx)
	if err != nil {
		t.Fatalf("ListSyncRecords: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("initial records = %d, want 0", len(records))
	}

	// Create a task to reference.
	task, err := store.Create(ctx, CreateParams{Title: "Test task"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Upsert a sync record.
	now := time.Now().UTC()
	r := &NotionSyncRecord{
		TaskID:         task.ID,
		NotionPageID:   "page-123",
		LastSyncedAt:   now,
		SyncDirection:  "both",
		ConflictStatus: "",
	}
	if err := store.UpsertSyncRecord(ctx, r); err != nil {
		t.Fatalf("UpsertSyncRecord: %v", err)
	}

	// Get by task ID.
	got, err := store.GetSyncRecord(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetSyncRecord: %v", err)
	}
	if got == nil {
		t.Fatal("expected sync record, got nil")
	}
	if got.NotionPageID != "page-123" {
		t.Errorf("page_id = %q, want page-123", got.NotionPageID)
	}
	if got.SyncDirection != "both" {
		t.Errorf("direction = %q, want both", got.SyncDirection)
	}

	// Get by page ID.
	got2, err := store.GetSyncRecordByPage(ctx, "page-123")
	if err != nil {
		t.Fatalf("GetSyncRecordByPage: %v", err)
	}
	if got2 == nil || got2.TaskID != task.ID {
		t.Error("GetSyncRecordByPage returned wrong record")
	}

	// Update (upsert).
	r.ConflictStatus = "status:conflict"
	if err := store.UpsertSyncRecord(ctx, r); err != nil {
		t.Fatalf("UpsertSyncRecord update: %v", err)
	}
	got3, _ := store.GetSyncRecord(ctx, task.ID)
	if got3.ConflictStatus != "status:conflict" {
		t.Errorf("conflict_status = %q, want status:conflict", got3.ConflictStatus)
	}

	// List should have 1.
	records, _ = store.ListSyncRecords(ctx)
	if len(records) != 1 {
		t.Errorf("records = %d, want 1", len(records))
	}

	// List conflicts.
	conflicts, _ := store.ListConflicts(ctx)
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}

	// Delete.
	if err := store.DeleteSyncRecord(ctx, task.ID); err != nil {
		t.Fatalf("DeleteSyncRecord: %v", err)
	}
	got4, _ := store.GetSyncRecord(ctx, task.ID)
	if got4 != nil {
		t.Error("sync record should be deleted")
	}
}

func TestSyncRecord_NotFound(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	got, err := store.GetSyncRecord(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSyncRecord: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent record")
	}
}

func TestSyncRecord_CascadeDelete(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	now := time.Now().UTC()
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  now,
		SyncDirection: "both",
	})

	// Delete the task — sync record should cascade.
	if err := store.DeleteTask(ctx, task.ID, "test"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	got, _ := store.GetSyncRecord(ctx, task.ID)
	if got != nil {
		t.Error("sync record should be cascade-deleted with task")
	}
}

func TestNotionClient_LookupUser(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)
	client.Users["Alice"] = notionapi.User{ID: "u1", Name: "Alice"}

	u, ok := client.LookupUser("Alice")
	if !ok || string(u.ID) != "u1" {
		t.Error("Alice not found")
	}

	_, ok = client.LookupUser("Unknown")
	if ok {
		t.Error("Unknown should not be found")
	}
}
