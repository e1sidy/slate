package slate

import (
	"context"
	"testing"
	"time"

	"github.com/jomei/notionapi"
)

func makePage(id string, props notionapi.Properties) notionapi.Page {
	return notionapi.Page{
		ID:             notionapi.ObjectID(id),
		LastEditedTime: time.Now().Add(time.Hour), // future = always "modified"
		Properties:     props,
	}
}

func TestPullChanges_UpdateExisting(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Original"})
	past := time.Now().Add(-time.Hour)
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  past,
		SyncDirection: "both",
	})

	page := makePage("page-1", notionapi.Properties{
		"Task name": &notionapi.TitleProperty{
			Type:  notionapi.PropertyTypeTitle,
			Title: []notionapi.RichText{{PlainText: "Updated title", Text: &notionapi.Text{Content: "Updated title"}}},
		},
		"Priority": &notionapi.SelectProperty{
			Type:   notionapi.PropertyTypeSelect,
			Select: notionapi.Option{Name: "High"},
		},
	})

	mock := &mockNotionAPI{
		queryFn: func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error) {
			return &notionapi.DatabaseQueryResponse{
				Results: []notionapi.Page{page},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	result, err := client.PullChanges(ctx, store)
	if err != nil {
		t.Fatalf("PullChanges: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("updated = %d, want 1", result.Updated)
	}

	// Verify task was updated.
	updated, _ := store.Get(ctx, task.ID)
	if updated.Title != "Updated title" {
		t.Errorf("title = %q, want Updated title", updated.Title)
	}
	if updated.Priority != P1 {
		t.Errorf("priority = %d, want %d (P1 from High)", updated.Priority, P1)
	}
}

func TestPullChanges_CreateNew(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	page := makePage("new-page-1", notionapi.Properties{
		"Task name": &notionapi.TitleProperty{
			Type:  notionapi.PropertyTypeTitle,
			Title: []notionapi.RichText{{PlainText: "New from Notion", Text: &notionapi.Text{Content: "New from Notion"}}},
		},
		"Status": &notionapi.StatusProperty{
			Type:   notionapi.PropertyTypeStatus,
			Status: notionapi.Status{Name: "In Progress"},
		},
	})

	mock := &mockNotionAPI{
		queryFn: func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error) {
			return &notionapi.DatabaseQueryResponse{
				Results: []notionapi.Page{page},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	result, err := client.PullChanges(ctx, store)
	if err != nil {
		t.Fatalf("PullChanges: %v", err)
	}
	if result.Created != 1 {
		t.Errorf("created = %d, want 1", result.Created)
	}

	// Verify sync record was created.
	rec, _ := store.GetSyncRecordByPage(ctx, "new-page-1")
	if rec == nil {
		t.Fatal("sync record not created")
	}

	// Verify task exists with correct status.
	task, _ := store.Get(ctx, rec.TaskID)
	if task == nil {
		t.Fatal("task not created")
	}
	if task.Title != "New from Notion" {
		t.Errorf("title = %q, want New from Notion", task.Title)
	}
	if task.Status != StatusInProgress {
		t.Errorf("status = %q, want in_progress", task.Status)
	}
}

func TestPullChanges_SkipUnmodified(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	future := time.Now().Add(time.Hour) // synced_at is in the future
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID:        task.ID,
		NotionPageID:  "page-1",
		LastSyncedAt:  future,
		SyncDirection: "both",
	})

	// Page last edited is before sync time.
	page := notionapi.Page{
		ID:             notionapi.ObjectID("page-1"),
		LastEditedTime: time.Now(), // before future sync time
		Properties:     notionapi.Properties{},
	}

	mock := &mockNotionAPI{
		queryFn: func(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error) {
			return &notionapi.DatabaseQueryResponse{Results: []notionapi.Page{page}}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	result, _ := client.PullChanges(ctx, store)
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
	if result.Updated != 0 {
		t.Errorf("updated = %d, want 0", result.Updated)
	}
}

func TestPropertiesToUpdate_StatusMapping(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	cfg.StatusMap["in_progress"] = []string{"In Progress", "In Review", "On QA"}

	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Status": &notionapi.StatusProperty{
				Type:   notionapi.PropertyTypeStatus,
				Status: notionapi.Status{Name: "In Review"},
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.status == nil {
		t.Fatal("status should be set")
	}
	if *result.status != StatusInProgress {
		t.Errorf("status = %q, want in_progress (In Review maps to in_progress)", *result.status)
	}
}

func TestPropertiesToUpdate_PriorityMapping(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Priority": &notionapi.SelectProperty{
				Type:   notionapi.PropertyTypeSelect,
				Select: notionapi.Option{Name: "High"},
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.params.Priority == nil {
		t.Fatal("priority should be set")
	}
	if *result.params.Priority != P1 {
		t.Errorf("priority = %d, want %d (High → P1)", *result.params.Priority, P1)
	}
}

func TestPropertiesToUpdate_Assignee(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Assignee": &notionapi.PeopleProperty{
				Type:   notionapi.PropertyTypePeople,
				People: []notionapi.User{{ID: "u1", Name: "Alice"}},
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.params.Assignee == nil {
		t.Fatal("assignee should be set")
	}
	if *result.params.Assignee != "Alice" {
		t.Errorf("assignee = %q, want Alice", *result.params.Assignee)
	}
}

func TestPropertiesToUpdate_Labels(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Tags": &notionapi.MultiSelectProperty{
				Type:        notionapi.PropertyTypeMultiSelect,
				MultiSelect: []notionapi.Option{{Name: "bug"}, {Name: "urgent"}},
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.params.Labels == nil {
		t.Fatal("labels should be set")
	}
	if len(*result.params.Labels) != 2 {
		t.Errorf("labels = %d, want 2", len(*result.params.Labels))
	}
}

func TestPropertiesToUpdate_ParentRelation(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Parent-task": &notionapi.RelationProperty{
				Type:     notionapi.PropertyTypeRelation,
				Relation: []notionapi.Relation{{ID: "parent-page-id"}},
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.parentPageID != "parent-page-id" {
		t.Errorf("parentPageID = %q, want parent-page-id", result.parentPageID)
	}
	if result.parentCleared {
		t.Error("parentCleared should be false")
	}
}

func TestPropertiesToUpdate_ParentCleared(t *testing.T) {
	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(&mockNotionAPI{}, &cfg)

	page := &notionapi.Page{
		Properties: notionapi.Properties{
			"Parent-task": &notionapi.RelationProperty{
				Type:     notionapi.PropertyTypeRelation,
				Relation: []notionapi.Relation{}, // empty = cleared
			},
		},
	}

	result := client.propertiesToUpdate(page)
	if result.parentPageID != "" {
		t.Errorf("parentPageID = %q, want empty", result.parentPageID)
	}
	if !result.parentCleared {
		t.Error("parentCleared should be true")
	}
}

func TestStripSlatePrefix(t *testing.T) {
	tt := []struct {
		input string
		want  string
	}{
		{"[st-ab12] Fix bug", "Fix bug"},
		{"[st-1234] Task", "Task"},
		{"No prefix", "No prefix"},
		{"[incomplete", "[incomplete"},
		{"", ""},
	}
	for _, tc := range tt {
		got := stripSlatePrefix(tc.input)
		if got != tc.want {
			t.Errorf("stripSlatePrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPullComments_NewComments(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})
	store.UpsertSyncRecord(ctx, &NotionSyncRecord{
		TaskID: task.ID, NotionPageID: "page-1",
		LastSyncedAt: timeNowUTC(), SyncDirection: "both",
	})

	now := time.Now()
	mock := &mockNotionAPI{
		commentsFn: func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error) {
			return &notionapi.CommentQueryResponse{
				Results: []notionapi.Comment{
					{
						RichText:    []notionapi.RichText{{PlainText: "Comment 1"}},
						CreatedTime: now,
					},
					{
						RichText:    []notionapi.RichText{{PlainText: "Comment 2"}},
						CreatedTime: now.Add(time.Minute),
					},
				},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	n, err := client.pullComments(ctx, store, "page-1", task.ID)
	if err != nil {
		t.Fatalf("pullComments: %v", err)
	}
	if n != 2 {
		t.Errorf("created = %d, want 2", n)
	}

	// Second pull should not create duplicates.
	n2, err := client.pullComments(ctx, store, "page-1", task.ID)
	if err != nil {
		t.Fatalf("pullComments 2: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second pull created = %d, want 0 (deduped)", n2)
	}

	comments, _ := store.ListComments(ctx, task.ID)
	if len(comments) != 2 {
		t.Errorf("total comments = %d, want 2", len(comments))
	}
}

func TestPullComments_EmptySkipped(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Test"})

	mock := &mockNotionAPI{
		commentsFn: func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error) {
			return &notionapi.CommentQueryResponse{
				Results: []notionapi.Comment{
					{RichText: []notionapi.RichText{}, CreatedTime: time.Now()}, // empty
				},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	n, _ := client.pullComments(ctx, store, "page-1", task.ID)
	if n != 0 {
		t.Errorf("created = %d, want 0 (empty skipped)", n)
	}
}

func TestReadPageBody(t *testing.T) {
	mock := &mockNotionAPI{
		getChildrenFn: func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error) {
			return &notionapi.GetChildrenResponse{
				Results: []notionapi.Block{
					&notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
						Paragraph:  notionapi.Paragraph{RichText: []notionapi.RichText{{PlainText: "Hello world"}}},
					},
					&notionapi.Heading2Block{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeHeading2},
						Heading2:   notionapi.Heading{RichText: []notionapi.RichText{{PlainText: "Section"}}},
					},
					&notionapi.BulletedListItemBlock{
						BasicBlock:       notionapi.BasicBlock{Type: notionapi.BlockTypeBulletedListItem},
						BulletedListItem: notionapi.ListItem{RichText: []notionapi.RichText{{PlainText: "Item 1"}}},
					},
				},
			}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	desc, err := client.readPageBody(context.Background(), "page-1")
	if err != nil {
		t.Fatalf("readPageBody: %v", err)
	}
	if desc == "" {
		t.Error("description should not be empty")
	}
	// Should contain all 3 text blocks.
	if !contains(desc, "Hello world") || !contains(desc, "Section") || !contains(desc, "Item 1") {
		t.Errorf("description missing content: %q", desc)
	}
}

func TestReadPageBody_Empty(t *testing.T) {
	mock := &mockNotionAPI{
		getChildrenFn: func(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error) {
			return &notionapi.GetChildrenResponse{Results: []notionapi.Block{}}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	desc, err := client.readPageBody(context.Background(), "page-1")
	if err != nil {
		t.Fatalf("readPageBody: %v", err)
	}
	if desc != "" {
		t.Errorf("empty page body should return empty string, got %q", desc)
	}
}

func TestRichTextToPlain(t *testing.T) {
	rts := []notionapi.RichText{
		{PlainText: "Hello "},
		{PlainText: "world"},
	}
	got := richTextToPlain(rts)
	if got != "Hello world" {
		t.Errorf("richTextToPlain = %q, want 'Hello world'", got)
	}
}

// contains is defined in dependency_test.go
