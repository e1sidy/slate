package slate

import (
	"context"
	"testing"
)

func TestSearchFTS_Basic(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Fix authentication bug"})
	store.Create(ctx, CreateParams{Title: "Add rate limiting"})
	store.Create(ctx, CreateParams{Title: "Update authentication docs"})

	results, err := store.SearchFTS(ctx, "authentication")
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("results = %d, want 2", len(results))
	}
}

func TestSearchFTS_Description(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Task A", Description: "Uses JWT tokens for auth"})
	store.Create(ctx, CreateParams{Title: "Task B", Description: "No relevant content"})

	results, err := store.SearchFTS(ctx, "JWT")
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %d, want 1", len(results))
	}
}

func TestSearchFTS_Empty(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	results, err := store.SearchFTS(ctx, "")
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if results != nil {
		t.Error("empty query should return nil")
	}
}

func TestSearchFTS_NoResults(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Hello world"})

	results, err := store.SearchFTS(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results = %d, want 0", len(results))
	}
}

func TestRebuildFTSIndex(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Test task"})

	if err := store.RebuildFTSIndex(ctx); err != nil {
		t.Fatalf("RebuildFTSIndex: %v", err)
	}

	// Search should still work after rebuild.
	results, err := store.SearchFTS(ctx, "test")
	if err != nil {
		t.Fatalf("SearchFTS after rebuild: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %d, want 1", len(results))
	}
}

func TestSearchFTS_UpdatedContent(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Original title"})

	// Update title — FTS trigger should update the index.
	newTitle := "Updated title with keyword"
	store.Update(ctx, task.ID, UpdateParams{Title: &newTitle}, "test")

	results, err := store.SearchFTS(ctx, "keyword")
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %d, want 1 (FTS should reflect updated title)", len(results))
	}

	// Old title should not match.
	results2, _ := store.SearchFTS(ctx, "Original")
	if len(results2) != 0 {
		t.Errorf("old title results = %d, want 0", len(results2))
	}
}
