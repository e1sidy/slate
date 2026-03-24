package slate

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestArchive_BasicFlow(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	// Create and close a task.
	task, _ := store.Create(ctx, CreateParams{Title: "Old task"})
	store.CloseTask(ctx, task.ID, "done", "user")

	// Also create an open task (should NOT be archived).
	store.Create(ctx, CreateParams{Title: "Active task"})

	archivePath := filepath.Join(t.TempDir(), "archive.db")

	// Archive with a future cutoff (archives everything closed).
	result, err := store.Archive(ctx, time.Now().Add(time.Hour), archivePath)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if result.Archived != 1 {
		t.Errorf("archived = %d, want 1", result.Archived)
	}

	// Verify task is gone from main DB.
	_, err = store.Get(ctx, task.ID)
	if err == nil {
		t.Error("archived task should not be in main DB")
	}

	// Verify task is in archive.
	archived, err := ListArchived(ctx, archivePath)
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if len(archived) != 1 {
		t.Errorf("archived list = %d, want 1", len(archived))
	}
}

func TestArchive_NothingToArchive(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	store.Create(ctx, CreateParams{Title: "Open task"})

	archivePath := filepath.Join(t.TempDir(), "archive.db")
	result, err := store.Archive(ctx, time.Now().Add(time.Hour), archivePath)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if result.Archived != 0 {
		t.Errorf("archived = %d, want 0", result.Archived)
	}
}

func TestUnarchive(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	task, _ := store.Create(ctx, CreateParams{Title: "Archived task"})
	store.AddComment(ctx, task.ID, "user", "A comment")
	store.CloseTask(ctx, task.ID, "done", "user")

	archivePath := filepath.Join(t.TempDir(), "archive.db")
	store.Archive(ctx, time.Now().Add(time.Hour), archivePath)

	// Restore.
	restored, err := store.Unarchive(ctx, archivePath, nil)
	if err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	if restored != 1 {
		t.Errorf("restored = %d, want 1", restored)
	}

	// Verify task is back.
	got, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get after unarchive: %v", err)
	}
	if got.Title != "Archived task" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestUnarchive_Specific(t *testing.T) {
	store := tempDB(t)
	ctx := context.Background()

	t1, _ := store.Create(ctx, CreateParams{Title: "Task 1"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Task 2"})
	store.CloseTask(ctx, t1.ID, "done", "user")
	store.CloseTask(ctx, t2.ID, "done", "user")

	archivePath := filepath.Join(t.TempDir(), "archive.db")
	store.Archive(ctx, time.Now().Add(time.Hour), archivePath)

	// Restore only t1.
	restored, err := store.Unarchive(ctx, archivePath, []string{t1.ID})
	if err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	if restored != 1 {
		t.Errorf("restored = %d, want 1", restored)
	}

	// t1 should be back, t2 still in archive.
	_, err = store.Get(ctx, t1.ID)
	if err != nil {
		t.Error("t1 should be restored")
	}
	_, err = store.Get(ctx, t2.ID)
	if err == nil {
		t.Error("t2 should still be archived")
	}
}

func TestListArchived_NoFile(t *testing.T) {
	tasks, err := ListArchived(context.Background(), "/nonexistent/archive.db")
	if err != nil {
		t.Fatalf("ListArchived: %v", err)
	}
	if tasks != nil {
		t.Error("should return nil for nonexistent file")
	}
}

func TestDefaultArchivePath(t *testing.T) {
	path := DefaultArchivePath()
	if path == "" {
		t.Error("archive path should not be empty")
	}
}
