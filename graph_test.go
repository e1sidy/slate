package slate

import (
	"context"
	"strings"
	"testing"
)

func TestDepMermaid_Basic(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	t1, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task B"})
	store.AddDependency(ctx, t1.ID, t2.ID, Blocks)

	result, err := store.DepMermaid(ctx, "")
	if err != nil {
		t.Fatalf("DepMermaid: %v", err)
	}
	if !strings.Contains(result, "graph TD") {
		t.Error("should contain 'graph TD' header")
	}
	if !strings.Contains(result, "Task A") {
		t.Error("should contain Task A")
	}
	if !strings.Contains(result, "blocks") {
		t.Error("should contain edge label 'blocks'")
	}
}

func TestDepMermaid_Scoped(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	parent, _ := store.Create(ctx, CreateParams{Title: "Epic"})
	child, _ := store.Create(ctx, CreateParams{Title: "Child", ParentID: parent.ID})
	_ = child
	store.Create(ctx, CreateParams{Title: "Unrelated"})

	result, err := store.DepMermaid(ctx, parent.ID)
	if err != nil {
		t.Fatalf("DepMermaid scoped: %v", err)
	}
	if !strings.Contains(result, "Epic") {
		t.Error("should contain Epic")
	}
	if strings.Contains(result, "Unrelated") {
		t.Error("should NOT contain Unrelated (out of scope)")
	}
}

func TestDepMermaid_Empty(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	result, err := store.DepMermaid(ctx, "")
	if err != nil {
		t.Fatalf("DepMermaid: %v", err)
	}
	if !strings.Contains(result, "No tasks") {
		t.Error("empty should show 'No tasks'")
	}
}

func TestDepDOT_Basic(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	t1, _ := store.Create(ctx, CreateParams{Title: "Task A"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task B"})
	store.AddDependency(ctx, t1.ID, t2.ID, Blocks)

	result, err := store.DepDOT(ctx, "")
	if err != nil {
		t.Fatalf("DepDOT: %v", err)
	}
	if !strings.Contains(result, "digraph") {
		t.Error("should contain 'digraph'")
	}
	if !strings.Contains(result, "->") {
		t.Error("should contain edge '->'")
	}
}

func TestDepMermaid_StatusStyles(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "In Progress"})
	store.Claim(ctx, task.ID, "agent")

	result, _ := store.DepMermaid(ctx, "")
	if !strings.Contains(result, "fill:#ffd700") {
		t.Error("in_progress should have gold fill style")
	}
}
