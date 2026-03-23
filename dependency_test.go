package slate

import (
	"testing"
)

func TestAddDependency(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})

	if err := store.AddDependency(ctx, a.ID, b.ID, Blocks); err != nil {
		t.Fatal(err)
	}

	deps, err := store.ListDependencies(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 {
		t.Fatalf("deps count = %d, want 1", len(deps))
	}
	if deps[0].ToID != b.ID {
		t.Errorf("dep toID = %q, want %s", deps[0].ToID, b.ID)
	}
}

func TestAddDependency_SelfDep(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	err := store.AddDependency(ctx, a.ID, a.ID, Blocks)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
}

func TestAddDependency_Duplicate(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})

	store.AddDependency(ctx, a.ID, b.ID, Blocks)
	err := store.AddDependency(ctx, a.ID, b.ID, Blocks)
	if err != nil {
		t.Fatalf("duplicate should be idempotent, got: %v", err)
	}
}

func TestAddDependency_CycleDetection(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})
	c, _ := store.Create(ctx, CreateParams{Title: "Task C"})

	store.AddDependency(ctx, a.ID, b.ID, Blocks)
	store.AddDependency(ctx, b.ID, c.ID, Blocks)

	// c -> a would create A -> B -> C -> A cycle
	err := store.AddDependency(ctx, c.ID, a.ID, Blocks)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestRemoveDependency(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})

	store.AddDependency(ctx, a.ID, b.ID, Blocks)
	if err := store.RemoveDependency(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	deps, _ := store.ListDependencies(ctx, a.ID)
	if len(deps) != 0 {
		t.Errorf("deps count = %d, want 0 after removal", len(deps))
	}
}

func TestListDependents(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})
	c, _ := store.Create(ctx, CreateParams{Title: "Task C"})

	store.AddDependency(ctx, a.ID, b.ID, Blocks)
	store.AddDependency(ctx, c.ID, b.ID, Blocks)

	deps, err := store.ListDependents(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Errorf("dependents count = %d, want 2", len(deps))
	}
}

func TestDepTree(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})

	store.AddDependency(ctx, a.ID, b.ID, Blocks)

	tree, err := store.DepTree(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tree == "" {
		t.Fatal("empty tree output")
	}
	if !contains(tree, "Task A") || !contains(tree, "Task B") {
		t.Errorf("tree should contain both tasks, got:\n%s", tree)
	}
}

func TestDetectCycles_NoCycles(t *testing.T) {
	store := tempDB(t)

	a, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	b, _ := store.Create(ctx, CreateParams{Title: "Task B"})
	store.AddDependency(ctx, a.ID, b.ID, Blocks)

	cycles, err := store.DetectCycles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %d", len(cycles))
	}
}

func TestAutoUnblock(t *testing.T) {
	store := tempDB(t)

	blocker, _ := store.Create(ctx, CreateParams{Title: "Blocker"})
	blocked, _ := store.Create(ctx, CreateParams{Title: "Blocked"})

	store.AddDependency(ctx, blocked.ID, blocker.ID, Blocks)
	store.UpdateStatus(ctx, blocked.ID, StatusBlocked, "tester")

	// Close blocker — blocked should auto-unblock.
	if err := store.CloseTask(ctx, blocker.ID, "done", "tester"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get(ctx, blocked.ID)
	if got.Status != StatusOpen {
		t.Errorf("status = %q, want open (auto-unblocked)", got.Status)
	}
}

func TestReady(t *testing.T) {
	store := tempDB(t)

	ready1, _ := store.Create(ctx, CreateParams{Title: "Ready 1"})
	ready2, _ := store.Create(ctx, CreateParams{Title: "Ready 2"})
	blocker, _ := store.Create(ctx, CreateParams{Title: "Blocker"})
	blocked, _ := store.Create(ctx, CreateParams{Title: "Blocked"})

	store.AddDependency(ctx, blocked.ID, blocker.ID, Blocks)
	store.UpdateStatus(ctx, blocked.ID, StatusBlocked, "tester")

	tasks, err := store.Ready(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	readyIDs := make(map[string]bool)
	for _, task := range tasks {
		readyIDs[task.ID] = true
	}

	if !readyIDs[ready1.ID] || !readyIDs[ready2.ID] {
		t.Error("ready tasks should include unblocked open tasks")
	}
	if readyIDs[blocked.ID] {
		t.Error("blocked task should not be in ready list")
	}
	_ = blocker
}

func TestBlocked(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Open"})
	blocker, _ := store.Create(ctx, CreateParams{Title: "Blocker"})
	blocked, _ := store.Create(ctx, CreateParams{Title: "Blocked"})

	store.AddDependency(ctx, blocked.ID, blocker.ID, Blocks)
	store.UpdateStatus(ctx, blocked.ID, StatusBlocked, "tester")

	tasks, err := store.Blocked(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("blocked count = %d, want 1", len(tasks))
	}
	if tasks[0].ID != blocked.ID {
		t.Errorf("blocked ID = %q, want %s", tasks[0].ID, blocked.ID)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
