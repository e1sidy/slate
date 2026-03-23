package slate

import "testing"

func TestAddCheckpoint(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	cp, err := store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{
		Done:      "Implemented auth flow",
		Decisions: "Used JWT",
		Next:      "Add tests",
		Blockers:  "None",
		Files:     []string{"auth.go", "auth_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cp.Done != "Implemented auth flow" {
		t.Errorf("done = %q", cp.Done)
	}
	if len(cp.Files) != 2 {
		t.Errorf("files count = %d, want 2", len(cp.Files))
	}
}

func TestAddCheckpoint_EmptyDone(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	_, err := store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{})
	if err == nil {
		t.Fatal("expected error for empty done")
	}
}

func TestLatestCheckpoint(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{Done: "First"})
	store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{Done: "Second"})

	cp, err := store.LatestCheckpoint(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cp.Done != "Second" {
		t.Errorf("latest done = %q, want Second", cp.Done)
	}
}

func TestListCheckpoints(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{Done: "First"})
	store.AddCheckpoint(ctx, task.ID, "agent", CheckpointParams{Done: "Second", Files: []string{"x.go"}})

	cps, err := store.ListCheckpoints(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 2 {
		t.Fatalf("count = %d, want 2", len(cps))
	}
	if cps[0].Done != "First" {
		t.Errorf("first done = %q, want First", cps[0].Done)
	}
	if len(cps[1].Files) != 1 {
		t.Errorf("second files = %d, want 1", len(cps[1].Files))
	}
}
