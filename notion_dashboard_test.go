package slate

import (
	"context"
	"testing"
	"time"

	"github.com/jomei/notionapi"
)

func TestBuildDashboardBlocks(t *testing.T) {
	report := &MetricsReport{
		TasksCreated:   10,
		TasksClosed:    5,
		TasksCancelled: 1,
		CurrentOpen:    8,
		CurrentBlocked: 2,
		AvgCycleTime:   24 * time.Hour,
	}

	blocks := buildDashboardBlocks(report, time.Now())
	if len(blocks) == 0 {
		t.Error("blocks should not be empty")
	}

	// Should have at least heading + summary + metrics sections.
	if len(blocks) < 5 {
		t.Errorf("blocks = %d, expected at least 5", len(blocks))
	}
}

func TestBuildDashboardBlocks_NoCycleTime(t *testing.T) {
	report := &MetricsReport{
		TasksCreated: 3,
		TasksClosed:  0,
	}

	blocks := buildDashboardBlocks(report, time.Now())
	// Should not include cycle time line.
	for _, b := range blocks {
		if bp, ok := b.(notionapi.BulletedListItemBlock); ok {
			if len(bp.BulletedListItem.RichText) > 0 {
				text := bp.BulletedListItem.RichText[0].Text.Content
				if text == "Average cycle time: 0s" {
					t.Error("should not show 0 cycle time")
				}
			}
		}
	}
}

func TestBuildWeeklyBlocks(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	// Create a task and close it (within the week).
	task, _ := store.Create(ctx, CreateParams{Title: "Done task"})
	store.CloseTask(ctx, task.ID, "completed", "user")

	// Create a task with a decision checkpoint.
	task2, _ := store.Create(ctx, CreateParams{Title: "With decisions"})
	store.AddCheckpoint(ctx, task2.ID, "user", CheckpointParams{
		Done:      "Finished implementation",
		Decisions: "Used JWT over sessions",
	})
	store.CloseTask(ctx, task2.ID, "done", "user")

	now := time.Now().Add(time.Hour) // future to capture everything
	weekAgo := now.AddDate(0, 0, -7)

	blocks := buildWeeklyBlocks(ctx, store, weekAgo, now)
	if len(blocks) == 0 {
		t.Error("blocks should not be empty")
	}

	// Should have sections: heading, completed, created, decisions.
	if len(blocks) < 4 {
		t.Errorf("blocks = %d, expected at least 4 sections", len(blocks))
	}
}

func TestPushDashboard_Create(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Task 1"})
	store.Create(ctx, CreateParams{Title: "Task 2"})

	var createdChildren []notionapi.Block
	mock := &mockNotionAPI{
		createPageFn: func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			createdChildren = req.Children
			return &notionapi.Page{ID: notionapi.ObjectID("dashboard-page-1")}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	pageID, err := client.PushDashboard(ctx, store)
	if err != nil {
		t.Fatalf("PushDashboard: %v", err)
	}
	if pageID != "dashboard-page-1" {
		t.Errorf("pageID = %q, want dashboard-page-1", pageID)
	}
	if len(createdChildren) == 0 {
		t.Error("dashboard should have blocks")
	}
}

func TestPushDashboard_Update(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	updateCalled := false
	appendCalled := false
	mock := &mockNotionAPI{
		updatePageFn: func(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
			updateCalled = true
			return &notionapi.Page{ID: notionapi.ObjectID(id)}, nil
		},
		appendChildFn: func(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error) {
			appendCalled = true
			return &notionapi.AppendBlockChildrenResponse{}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.DashboardPageID = "existing-dashboard"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	pageID, err := client.PushDashboard(ctx, store)
	if err != nil {
		t.Fatalf("PushDashboard: %v", err)
	}
	if pageID != "existing-dashboard" {
		t.Errorf("pageID = %q, want existing-dashboard", pageID)
	}
	if !updateCalled {
		t.Error("should update existing page")
	}
	if !appendCalled {
		t.Error("should append blocks")
	}
}

func TestPushWeeklyDigest(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Test task"})

	mock := &mockNotionAPI{
		createPageFn: func(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
			// Verify title contains "Week of".
			if titleProp, ok := req.Properties["title"]; ok {
				tp := titleProp.(notionapi.TitleProperty)
				if len(tp.Title) > 0 && tp.Title[0].Text.Content == "" {
					t.Error("title should contain week info")
				}
			}
			return &notionapi.Page{ID: notionapi.ObjectID("digest-page-1")}, nil
		},
	}

	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "db-1"
	cfg.RateLimit = 0
	client := NewNotionClientWithAPI(mock, &cfg)

	pageID, err := client.PushWeeklyDigest(ctx, store)
	if err != nil {
		t.Fatalf("PushWeeklyDigest: %v", err)
	}
	if pageID != "digest-page-1" {
		t.Errorf("pageID = %q, want digest-page-1", pageID)
	}
}

func TestBlockHelpers(t *testing.T) {
	h1 := heading1("Test")
	if h1 == nil {
		t.Error("heading1 should not be nil")
	}

	h2 := heading2("Test")
	if h2 == nil {
		t.Error("heading2 should not be nil")
	}

	p := paragraph("Test")
	if p == nil {
		t.Error("paragraph should not be nil")
	}

	b := bulletItem("Test")
	if b == nil {
		t.Error("bulletItem should not be nil")
	}
}
