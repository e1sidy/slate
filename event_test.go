package slate

import (
	"testing"
	"time"
)

func TestEvents(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Tracked"})
	store.UpdateStatus(ctx, task.ID, StatusInProgress, "agent-1")
	store.CloseTask(ctx, task.ID, "done", "agent-1")

	events, err := store.Events(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	// created + status_changed(in_progress) + closed
	if len(events) < 3 {
		t.Errorf("events count = %d, want >= 3", len(events))
	}
}

func TestEventsSince(t *testing.T) {
	store := tempDB(t)

	task, _ := store.Create(ctx, CreateParams{Title: "Tracked"})

	// Use a time before creation to ensure we catch all events.
	since := task.CreatedAt.Add(-1 * time.Second)
	store.UpdateStatus(ctx, task.ID, StatusInProgress, "agent-1")

	events, err := store.EventsSince(ctx, task.ID, since)
	if err != nil {
		t.Fatal(err)
	}
	// Should include created + status_changed
	if len(events) < 2 {
		t.Errorf("events since count = %d, want >= 2", len(events))
	}
}
