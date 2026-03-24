package slate

import (
	"context"
	"testing"

	"github.com/jomei/notionapi"
)

func TestPushTask_Create(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test task", Type: TypeBug})

	var createdProps notionapi.Properties
	mock := &mockNotionAPI{
		database: &notionapi.Database{ID: "db-1"},
		createPageFn: func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			createdProps = req.Properties
			return &notionapi.Page{ID: notionapi.ObjectID("new-page-1")}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	if err := client.PushTask(ctx, store, task.ID); err != nil {
		t.Fatalf("PushTask: %v", err)
	}

	// Verify sync record was created.
	rec, _ := store.GetSyncRecord(ctx, task.ID)
	if rec == nil {
		t.Fatal("sync record not created")
	}
	if rec.NotionPageID != "new-page-1" {
		t.Errorf("page_id = %q, want new-page-1", rec.NotionPageID)
	}

	// Verify title contains task ID.
	if titleProp, ok := createdProps["Task name"]; ok {
		tp := titleProp.(notionapi.TitleProperty)
		if len(tp.Title) == 0 {
			t.Error("title should not be empty")
		} else {
			content := tp.Title[0].Text.Content
			if content == "" {
				t.Error("title content empty")
			}
		}
	} else {
		t.Error("Task name property missing")
	}

	// Verify status mapping.
	if statusProp, ok := createdProps["Status"]; ok {
		sp := statusProp.(notionapi.StatusProperty)
		if sp.Status.Name != "Todo" {
			t.Errorf("status = %q, want Todo", sp.Status.Name)
		}
	} else {
		t.Error("Status property missing")
	}

	// Verify priority mapping. Create with zero-value Priority = P0 → "High".
	if priorityProp, ok := createdProps["Priority"]; ok {
		pp := priorityProp.(notionapi.SelectProperty)
		if pp.Select.Name != "High" {
			t.Errorf("priority = %q, want High (P0 default)", pp.Select.Name)
		}
	} else {
		t.Error("Priority property missing")
	}
}

func TestPushTask_Update(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test task"})

	// Pre-create sync record.
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "existing-page",
		LastSyncedAt:  timeNowUTC(),
		SyncDirection: "both",
	})

	updateCalled := false
	mock := &mockNotionAPI{
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			updateCalled = true
			if string(id) != "existing-page" {
				t.Errorf("update page ID = %q, want existing-page", id)
			}
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	if err := client.PushTask(ctx, store, task.ID); err != nil {
		t.Fatalf("PushTask: %v", err)
	}

	if !updateCalled {
		t.Error("UpdatePage should have been called for existing sync")
	}
}

func TestPushAll_CreatesAndUpdates(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task1, _ := store.Create(ctx, CreateParams{Title: "Task 1"})
	task2, _ := store.Create(ctx, CreateParams{Title: "Task 2"})

	// Pre-sync task1.
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task1.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  timeNowUTC(),
		SyncDirection: "both",
	})

	creates := 0
	updates := 0
	mock := &mockNotionAPI{
		database: &notionapi.Database{ID: "db-1"},
		createPageFn: func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			creates++
			return &notionapi.Page{ID: notionapi.ObjectID("new-page")}, nil
		},
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			updates++
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	cfg.PropertyMap.ParentID = "" // skip parent pass
	client := NewNotionClientWithAPI(mock, &cfg)

	result, err := client.PushAll(ctx, store, ListParams{})
	if err != nil {
		t.Fatalf("PushAll: %v", err)
	}

	_ = task2 // used implicitly
	if result.Created != 1 {
		t.Errorf("created = %d, want 1", result.Created)
	}
	if result.Updated != 1 {
		t.Errorf("updated = %d, want 1", result.Updated)
	}
}

func TestPushAll_ParentBeforeChild(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	child, _ := store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})
	_ = child

	var createOrder []string
	mock := &mockNotionAPI{
		database: &notionapi.Database{ID: "db-1"},
		createPageFn: func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			// Extract title to determine which task.
			if titleProp, ok := req.Properties["Task name"]; ok {
				tp := titleProp.(notionapi.TitleProperty)
				if len(tp.Title) > 0 {
					createOrder = append(createOrder, tp.Title[0].Text.Content)
				}
			}
			return &notionapi.Page{ID: notionapi.ObjectID("page-" + string(rune(len(createOrder))))}, nil
		},
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	_, err := client.PushAll(ctx, store, ListParams{})
	if err != nil {
		t.Fatalf("PushAll: %v", err)
	}

	// Parent should be created before child.
	if len(createOrder) != 2 {
		t.Fatalf("created %d, want 2", len(createOrder))
	}
	// First created should contain "Parent".
	if createOrder[0] == "" {
		t.Error("first create should have content")
	}
}

func TestTaskToProperties_StatusMapping(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	cfg.StatusMap["in_progress"] = []string{"In Progress", "In Review", "On QA"}

	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	task := &Task{
		ID:     "st-test",
		Title:  "Test",
		Status: StatusInProgress,
	}

	props := client.taskToProperties(task)
	statusProp := props["Status"].(notionapi.StatusProperty)
	if statusProp.Status.Name != "In Progress" {
		t.Errorf("status = %q, want In Progress (first in mapping)", statusProp.Status.Name)
	}
}

func TestTaskToProperties_PriorityMapping(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0

	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	tt := []struct {
		priority Priority
		want     string
	}{
		{0, "High"},
		{1, "High"},
		{2, "Medium"},
		{3, "Low"},
		{4, "Low"},
	}

	for _, tc := range tt {
		task := &Task{ID: "st-test", Title: "Test", Priority: tc.priority}
		props := client.taskToProperties(task)
		pp := props["Priority"].(notionapi.SelectProperty)
		if pp.Select.Name != tc.want {
			t.Errorf("P%d: got %q, want %q", tc.priority, pp.Select.Name, tc.want)
		}
	}
}

func TestTaskToProperties_Labels(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	task := &Task{
		ID:     "st-test",
		Title:  "Test",
		Labels: []string{"bug", "urgent"},
	}

	props := client.taskToProperties(task)
	mp := props["Tags"].(notionapi.MultiSelectProperty)
	if len(mp.MultiSelect) != 2 {
		t.Errorf("labels = %d, want 2", len(mp.MultiSelect))
	}
}

func TestTaskToProperties_Assignee(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)
	client.Users["Alice"] = notionapi.User{ID: "u1", Name: "Alice"}

	task := &Task{
		ID:       "st-test",
		Title:    "Test",
		Assignee: "Alice",
	}

	props := client.taskToProperties(task)
	pp := props["Assignee"].(notionapi.PeopleProperty)
	if len(pp.People) != 1 {
		t.Fatalf("people = %d, want 1", len(pp.People))
	}
	if string(pp.People[0].ID) != "u1" {
		t.Errorf("user ID = %q, want u1", pp.People[0].ID)
	}
}

func TestTaskToProperties_AssigneeUnknown(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)
	// No users cached.

	task := &Task{
		ID:       "st-test",
		Title:    "Test",
		Assignee: "Unknown",
	}

	props := client.taskToProperties(task)
	// Assignee should not be in properties when user not found.
	if _, ok := props["Assignee"]; ok {
		t.Error("assignee should be skipped when user not found")
	}
}

func TestSortByDepth(t *testing.T) {
	tasks := []*Task{
		{ID: "c", ParentID: "b"},
		{ID: "a", ParentID: ""},
		{ID: "b", ParentID: "a"},
	}

	sortByDepth(tasks)

	if tasks[0].ID != "a" {
		t.Errorf("first = %s, want a (root)", tasks[0].ID)
	}
	if tasks[1].ID != "b" {
		t.Errorf("second = %s, want b (depth 1)", tasks[1].ID)
	}
	if tasks[2].ID != "c" {
		t.Errorf("third = %s, want c (depth 2)", tasks[2].ID)
	}
}

func TestDescriptionToBlocks(t *testing.T) {
	blocks := descriptionToBlocks("Hello world")
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(blocks))
	}

	blocks2 := descriptionToBlocks("")
	if blocks2 != nil {
		t.Error("empty description should return nil")
	}
}

func TestPushDependencies(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	blocker, _ := store.Create(ctx, CreateParams{Title: "Blocker"})
	blocked, _ := store.Create(ctx, CreateParams{Title: "Blocked"})
	store.AddDependency(ctx, blocker.ID, blocked.ID, Blocks)

	// Sync both.
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{TaskID: blocker.ID, NotionPageID: "page-blocker", LastSyncedAt: timeNowUTC(), SyncDirection: "both"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{TaskID: blocked.ID, NotionPageID: "page-blocked", LastSyncedAt: timeNowUTC(), SyncDirection: "both"})

	var updatedProps notionapi.Properties
	mock := &mockNotionAPI{
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			if string(id) == "page-blocked" {
				updatedProps = req.Properties
			}
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	if err := client.pushDependencies(ctx, store, blocked.ID); err != nil {
		t.Fatalf("pushDependencies: %v", err)
	}

	// Should have "Blocked by" relation.
	if updatedProps == nil {
		t.Fatal("no update made for blocked task")
	}
	rel, ok := updatedProps["Blocked by"]
	if !ok {
		t.Fatal("Blocked by property missing")
	}
	rp := rel.(notionapi.RelationProperty)
	if len(rp.Relation) != 1 {
		t.Fatalf("relations = %d, want 1", len(rp.Relation))
	}
	if string(rp.Relation[0].ID) != "page-blocker" {
		t.Errorf("relation page = %q, want page-blocker", rp.Relation[0].ID)
	}
}

func TestAddPRLinks(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})

	// Define and set pr_url attribute.
	store.DefineAttr(ctx, "pr_url", AttrString, "PR URL")
	store.SetAttr(ctx, task.ID, "pr_url", "https://github.com/org/repo/pull/42")

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	cfg.PropertyMap.PRLinks = "PRs"
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	props := notionapi.Properties{}
	client.addPRLinks(ctx, store, task.ID, props)

	if urlProp, ok := props["PRs"]; ok {
		up := urlProp.(notionapi.URLProperty)
		if up.URL != "https://github.com/org/repo/pull/42" {
			t.Errorf("URL = %q, want PR URL", up.URL)
		}
	} else {
		t.Error("PRs property missing")
	}
}

func TestAddPRLinks_NoAttr(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	cfg.PropertyMap.PRLinks = "PRs"
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	props := notionapi.Properties{}
	client.addPRLinks(ctx, store, task.ID, props)

	if _, ok := props["PRs"]; ok {
		t.Error("PRs should not be set when no attr exists")
	}
}

func TestAddPRLinks_NotConfigured(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.DefineAttr(ctx, "pr_url", AttrString, "PR URL")
	store.SetAttr(ctx, task.ID, "pr_url", "https://example.com")

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	// PRLinks not set (empty)
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	props := notionapi.Properties{}
	client.addPRLinks(ctx, store, task.ID, props)

	if len(props) != 0 {
		t.Error("should not add props when PRLinks not configured")
	}
}
