package slate

import (
	"context"
	"testing"
	"time"
)

var ctx = context.Background()

func TestCreate_Basic(t *testing.T) {
	store := tempDB(t)

	task, err := store.Create(ctx, CreateParams{Title: "Test task"})
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Test task" {
		t.Errorf("title = %q, want Test task", task.Title)
	}
	if task.Status != StatusOpen {
		t.Errorf("status = %q, want open", task.Status)
	}
	if task.Type != TypeTask {
		t.Errorf("type = %q, want task", task.Type)
	}
	// Default priority is P0 (Go zero-value for int)
	if task.Priority != P0 {
		t.Errorf("priority = %d, want 0 (critical/default)", task.Priority)
	}
}

func TestCreate_WithAllFields(t *testing.T) {
	store := tempDB(t)

	due := time.Now().Add(24 * time.Hour)
	task, err := store.Create(ctx, CreateParams{
		Title:       "Full task",
		Description: "A description",
		Type:        TypeBug,
		Priority:    P1,
		Assignee:    "alice",
		Labels:      []string{"api", "urgent"},
		Notes:       "some notes",
		Estimate:    120,
		DueAt:       &due,
		CreatedBy:   "agent-1",
		Metadata:    `{"key":"value"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Type != TypeBug {
		t.Errorf("type = %q, want bug", task.Type)
	}
	if task.Priority != P1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	if task.Assignee != "alice" {
		t.Errorf("assignee = %q, want alice", task.Assignee)
	}
	if len(task.Labels) != 2 {
		t.Errorf("labels count = %d, want 2", len(task.Labels))
	}
}

func TestCreate_EmptyTitle(t *testing.T) {
	store := tempDB(t)

	_, err := store.Create(ctx, CreateParams{})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreate_WithParent(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	child1, err := store.Create(ctx, CreateParams{Title: "Child 1", ParentID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if child1.ID != parent.ID+".1" {
		t.Errorf("child1 ID = %q, want %s.1", child1.ID, parent.ID)
	}
	if child1.ParentID != parent.ID {
		t.Errorf("child1 parentID = %q, want %s", child1.ParentID, parent.ID)
	}

	child2, _ := store.Create(ctx, CreateParams{Title: "Child 2", ParentID: parent.ID})
	if child2.ID != parent.ID+".2" {
		t.Errorf("child2 ID = %q, want %s.2", child2.ID, parent.ID)
	}
}

func TestCreate_InvalidParent(t *testing.T) {
	store := tempDB(t)

	_, err := store.Create(ctx, CreateParams{Title: "Orphan", ParentID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for invalid parent")
	}
}

func TestGet(t *testing.T) {
	store := tempDB(t)

	created, _ := store.Create(ctx, CreateParams{Title: "Fetch me"})
	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Fetch me" {
		t.Errorf("title = %q, want Fetch me", got.Title)
	}
}

func TestGet_NotFound(t *testing.T) {
	store := tempDB(t)

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestUpdate_Partial(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Original", Priority: P2})
	newTitle := "Updated"
	updated, err := store.Update(ctx, task.ID, UpdateParams{Title: &newTitle}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" {
		t.Errorf("title = %q, want Updated", updated.Title)
	}
	if updated.Priority != P2 {
		t.Errorf("priority changed to %d, want 2 (unchanged)", updated.Priority)
	}
}

func TestUpdate_Assignee(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Assign me"})
	assignee := "bob"
	updated, err := store.Update(ctx, task.ID, UpdateParams{Assignee: &assignee}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Assignee != "bob" {
		t.Errorf("assignee = %q, want bob", updated.Assignee)
	}
}

func TestUpdateStatus(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Status test"})
	if err := store.UpdateStatus(ctx, task.ID, StatusInProgress, "tester"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(ctx, task.ID)
	if got.Status != StatusInProgress {
		t.Errorf("status = %q, want in_progress", got.Status)
	}
}

func TestCloseTask_Basic(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Close me"})
	if err := store.CloseTask(ctx, task.ID, "done", "tester"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(ctx, task.ID)
	if got.Status != StatusClosed {
		t.Errorf("status = %q, want closed", got.Status)
	}
	if got.CloseReason != "done" {
		t.Errorf("close_reason = %q, want done", got.CloseReason)
	}
	if got.ClosedAt == nil {
		t.Error("closed_at is nil, want non-nil")
	}
}

func TestCloseTask_BlockedByChildren(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})

	err := store.CloseTask(ctx, parent.ID, "done", "tester")
	if err == nil {
		t.Fatal("expected error when closing parent with open child")
	}
}

func TestCancelTask_Cascade(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	child, _ := store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})

	if err := store.CancelTask(ctx, parent.ID, "descoped", "tester"); err != nil {
		t.Fatal(err)
	}

	gotParent, _ := store.Get(ctx, parent.ID)
	if gotParent.Status != StatusCancelled {
		t.Errorf("parent status = %q, want cancelled", gotParent.Status)
	}

	gotChild, _ := store.Get(ctx, child.ID)
	if gotChild.Status != StatusCancelled {
		t.Errorf("child status = %q, want cancelled (cascade)", gotChild.Status)
	}
}

func TestReopen(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Reopen me"})
	store.CloseTask(ctx, task.ID, "done", "tester")

	if err := store.Reopen(ctx, task.ID, "tester"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(ctx, task.ID)
	if got.Status != StatusOpen {
		t.Errorf("status = %q, want open", got.Status)
	}
	if got.ClosedAt != nil {
		t.Error("closed_at should be nil after reopen")
	}
}

func TestReopen_NotTerminal(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Still open"})
	err := store.Reopen(ctx, task.ID, "tester")
	if err == nil {
		t.Fatal("expected error reopening non-terminal task")
	}
}

func TestDeleteTask(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})

	if err := store.DeleteTask(ctx, parent.ID, "tester"); err != nil {
		t.Fatal(err)
	}

	_, err := store.Get(ctx, parent.ID)
	if err == nil {
		t.Fatal("expected error getting deleted task")
	}
}

func TestClaim_Basic(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Claim me"})
	result, err := store.Claim(ctx, task.ID, "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get(ctx, task.ID)
	if got.Assignee != "agent-1" {
		t.Errorf("assignee = %q, want agent-1", got.Assignee)
	}
	if got.Status != StatusInProgress {
		t.Errorf("status = %q, want in_progress", got.Status)
	}
	_ = result
}

func TestClaim_DoubleClaim(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Contested"})
	_, err := store.Claim(ctx, task.ID, "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	// Second agent tries to claim — should fail.
	_, err = store.Claim(ctx, task.ID, "agent-2")
	if err != ErrAlreadyClaimed {
		t.Errorf("expected ErrAlreadyClaimed, got %v", err)
	}

	// Verify original claim is intact.
	got, _ := store.Get(ctx, task.ID)
	if got.Assignee != "agent-1" {
		t.Errorf("assignee = %q, want agent-1 (original)", got.Assignee)
	}
}

func TestClaim_SameAgentReClaim(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Re-claim"})
	store.Claim(ctx, task.ID, "agent-1")

	// Same agent re-claiming should succeed.
	_, err := store.Claim(ctx, task.ID, "agent-1")
	if err != nil {
		t.Errorf("same agent re-claim should succeed, got %v", err)
	}
}

func TestClaim_TerminalTask(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Closed"})
	store.CloseTask(ctx, task.ID, "done", "tester")

	_, err := store.Claim(ctx, task.ID, "agent-1")
	if err == nil {
		t.Fatal("expected error claiming closed task")
	}
}

func TestReleaseClaim(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Release me"})
	store.Claim(ctx, task.ID, "agent-1")

	if err := store.ReleaseClaim(ctx, task.ID, "agent-1"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get(ctx, task.ID)
	if got.Status != StatusOpen {
		t.Errorf("status = %q, want open", got.Status)
	}
	if got.Assignee != "" {
		t.Errorf("assignee = %q, want empty", got.Assignee)
	}
}

func TestExpireLeases(t *testing.T) {
	store := tempDB(t)
	store.leaseTimeout = 1 * time.Second

	task, _ := store.Create(ctx, CreateParams{Title: "Stale claim"})
	store.Claim(ctx, task.ID, "agent-1")

	// RFC3339 has second precision — need to wait >1 second for comparison to work.
	time.Sleep(2 * time.Second)

	n, err := store.ExpireLeases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expired %d leases, want 1", n)
	}

	got, _ := store.Get(ctx, task.ID)
	if got.Status != StatusOpen {
		t.Errorf("status = %q, want open (lease expired)", got.Status)
	}
	if got.Assignee != "" {
		t.Errorf("assignee = %q, want empty (lease expired)", got.Assignee)
	}
}

func TestAutoProgressParent(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	child, _ := store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})

	// Claim child — parent should auto-progress.
	result, err := store.Claim(ctx, child.ID, "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.ParentProgressed {
		t.Error("expected parent to auto-progress")
	}

	got, _ := store.Get(ctx, parent.ID)
	if got.Status != StatusInProgress {
		t.Errorf("parent status = %q, want in_progress", got.Status)
	}
}

func TestAutoProgressParent_NoRegress(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Parent"})
	child1, _ := store.Create(ctx, CreateParams{Title: "Child 1", ParentID: parent.ID})
	child2, _ := store.Create(ctx, CreateParams{Title: "Child 2", ParentID: parent.ID})

	store.Claim(ctx, child1.ID, "agent-1") // parent → in_progress

	// Claim second child — parent should stay in_progress (not regress).
	result, _ := store.Claim(ctx, child2.ID, "agent-2")
	if result.ParentProgressed {
		t.Error("parent should NOT re-progress (already in_progress)")
	}

	got, _ := store.Get(ctx, parent.ID)
	if got.Status != StatusInProgress {
		t.Errorf("parent status = %q, want in_progress", got.Status)
	}
}
