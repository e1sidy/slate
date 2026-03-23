package slate

import (
	"testing"
)

func TestMetrics(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Task 1"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task 2"})
	store.CloseTask(ctx, t2.ID, "done", "agent-1")

	report, err := store.Metrics(ctx, MetricsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if report.TasksCreated < 2 {
		t.Errorf("created = %d, want >= 2", report.TasksCreated)
	}
	if report.TasksClosed < 1 {
		t.Errorf("closed = %d, want >= 1", report.TasksClosed)
	}
	if report.CurrentOpen < 1 {
		t.Errorf("open = %d, want >= 1", report.CurrentOpen)
	}
}

func TestCycleTime(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Timed"})
	store.CloseTask(ctx, task.ID, "done", "tester")

	dur, err := store.CycleTime(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dur < 0 {
		t.Errorf("cycle time = %v, want >= 0", dur)
	}
}

func TestCycleTime_NotClosed(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Open"})
	_, err := store.CycleTime(ctx, task.ID)
	if err == nil {
		t.Error("expected error for unclosed task")
	}
}

func TestThroughput(t *testing.T) {
	store := tempDB(t)

	t1, _ := store.Create(ctx, CreateParams{Title: "Task 1"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task 2"})
	store.CloseTask(ctx, t1.ID, "done", "tester")
	store.CloseTask(ctx, t2.ID, "done", "tester")

	from := t1.CreatedAt.Add(-1)
	to := timeNowUTC().Add(1)
	count, err := store.Throughput(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if count < 2 {
		t.Errorf("throughput = %d, want >= 2", count)
	}
}

func TestNext(t *testing.T) {
	store := tempDB(t)

	// a blocks b, so closing a unblocks more work — a should be recommended.
	a, _ := store.Create(ctx, CreateParams{Title: "Unblocks B"})
	b, _ := store.Create(ctx, CreateParams{Title: "Blocked by A"})
	store.Create(ctx, CreateParams{Title: "Independent"})

	store.AddDependency(ctx, b.ID, a.ID, Blocks)
	store.UpdateStatus(ctx, b.ID, StatusBlocked, "tester")

	task, err := store.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != a.ID {
		t.Errorf("next = %s, want %s (unblocks more)", task.ID, a.ID)
	}
}

func TestNext_NoReady(t *testing.T) {
	store := tempDB(t)

	_, err := store.Next(ctx)
	if err == nil {
		t.Error("expected error when no ready tasks")
	}
}

func TestCloseMany(t *testing.T) {
	store := tempDB(t)

	t1, _ := store.Create(ctx, CreateParams{Title: "Task 1"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task 2"})

	n, err := store.CloseMany(ctx, []string{t1.ID, t2.ID}, "batch", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("closed = %d, want 2", n)
	}

	g1, _ := store.Get(ctx, t1.ID)
	g2, _ := store.Get(ctx, t2.ID)
	if g1.Status != StatusClosed || g2.Status != StatusClosed {
		t.Error("both should be closed")
	}
}

func TestUpdateMany(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Chore 1", Type: TypeChore})
	store.Create(ctx, CreateParams{Title: "Chore 2", Type: TypeChore})
	store.Create(ctx, CreateParams{Title: "Bug", Type: TypeBug})

	chore := TypeChore
	newAssignee := "bot"
	n, err := store.UpdateMany(ctx,
		ListParams{Type: &chore},
		UpdateParams{Assignee: &newAssignee},
		"tester",
	)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("updated = %d, want 2", n)
	}
}
