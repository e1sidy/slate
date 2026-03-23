package slate

import (
	"bytes"
	"strings"
	"testing"
)

func TestExportImportRoundTrip(t *testing.T) {
	store1 := tempDB(t)

	// Create test data.
	task1, _ := store1.Create(ctx, CreateParams{Title: "Task 1", Type: TypeBug, Priority: P1})
	task2, _ := store1.Create(ctx, CreateParams{Title: "Task 2", ParentID: task1.ID})
	store1.AddComment(ctx, task1.ID, "alice", "a comment")
	store1.AddDependency(ctx, task2.ID, task1.ID, Blocks)
	store1.AddCheckpoint(ctx, task1.ID, "agent", CheckpointParams{Done: "Stuff", Files: []string{"a.go"}})

	// Export.
	var buf bytes.Buffer
	if err := store1.ExportJSONL(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	exported := buf.String()
	if !strings.Contains(exported, "Task 1") {
		t.Error("export missing Task 1")
	}
	if !strings.Contains(exported, "a comment") {
		t.Error("export missing comment")
	}
	if !strings.Contains(exported, "checkpoint") {
		t.Error("export missing checkpoint")
	}

	// Import into new store.
	store2 := tempDB(t)
	if err := store2.ImportJSONL(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Verify tasks imported.
	got, err := store2.Get(ctx, task1.ID)
	if err != nil {
		t.Fatalf("import task1: %v", err)
	}
	if got.Title != "Task 1" {
		t.Errorf("imported title = %q, want Task 1", got.Title)
	}
	if got.Type != TypeBug {
		t.Errorf("imported type = %q, want bug", got.Type)
	}
}

func TestExportEvents(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Task"})

	var buf bytes.Buffer
	if err := store.ExportEvents(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "created") {
		t.Error("events export missing 'created' event")
	}
}
