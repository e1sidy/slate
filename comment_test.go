package slate

import "testing"

func TestAddComment(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	c, err := store.AddComment(ctx, task.ID, "alice", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if c.Content != "hello world" {
		t.Errorf("content = %q, want hello world", c.Content)
	}
	if c.Author != "alice" {
		t.Errorf("author = %q, want alice", c.Author)
	}
}

func TestAddComment_EmptyContent(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	_, err := store.AddComment(ctx, task.ID, "alice", "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestEditComment(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})
	c, _ := store.AddComment(ctx, task.ID, "alice", "original")

	if err := store.EditComment(ctx, c.ID, "updated"); err != nil {
		t.Fatal(err)
	}

	comments, _ := store.ListComments(ctx, task.ID)
	if len(comments) != 1 {
		t.Fatalf("comments count = %d, want 1", len(comments))
	}
	if comments[0].Content != "updated" {
		t.Errorf("content = %q, want updated", comments[0].Content)
	}
}

func TestDeleteComment(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})
	c, _ := store.AddComment(ctx, task.ID, "alice", "temp")

	if err := store.DeleteComment(ctx, c.ID); err != nil {
		t.Fatal(err)
	}

	comments, _ := store.ListComments(ctx, task.ID)
	if len(comments) != 0 {
		t.Errorf("comments count = %d, want 0", len(comments))
	}
}

func TestListComments_Order(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})
	store.AddComment(ctx, task.ID, "a", "first")
	store.AddComment(ctx, task.ID, "b", "second")

	comments, err := store.ListComments(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("count = %d, want 2", len(comments))
	}
	if comments[0].Content != "first" {
		t.Errorf("first = %q, want first", comments[0].Content)
	}
}
