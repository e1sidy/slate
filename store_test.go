package slate

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "test.db"), WithPrefix("st"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpen(t *testing.T) {
	store := tempDB(t)
	if store.DB() == nil {
		t.Fatal("DB() returned nil")
	}
	if store.Prefix() != "st" {
		t.Errorf("prefix = %q, want st", store.Prefix())
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "test.db")
	store, err := Open(context.Background(), path, WithPrefix("st"))
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
}

func TestGenerateID(t *testing.T) {
	id := GenerateID("st", 4)
	if !strings.HasPrefix(id, "st-") {
		t.Errorf("ID %q does not start with st-", id)
	}
	// prefix + "-" + 4 hex chars = 7 chars
	if len(id) != 7 {
		t.Errorf("ID %q length = %d, want 7", id, len(id))
	}
}

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := GenerateID("st", 6)
		if seen[id] {
			t.Fatalf("duplicate ID after %d iterations: %s", i, id)
		}
		seen[id] = true
	}
}

func TestGenerateID_HashLengthBounds(t *testing.T) {
	// Too short — should default to 4
	id := GenerateID("st", 1)
	if len(id) != 7 { // st- + 4
		t.Errorf("short hashLen: ID %q length = %d, want 7", id, len(id))
	}

	// Too long — should default to 4
	id = GenerateID("st", 20)
	if len(id) != 7 {
		t.Errorf("long hashLen: ID %q length = %d, want 7", id, len(id))
	}

	// Valid custom length
	id = GenerateID("st", 6)
	if len(id) != 9 { // st- + 6
		t.Errorf("custom hashLen: ID %q length = %d, want 9", id, len(id))
	}
}

func TestWithOptions(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "test.db"),
		WithPrefix("myapp"),
		WithHashLength(6),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if store.Prefix() != "myapp" {
		t.Errorf("prefix = %q, want myapp", store.Prefix())
	}
	id := store.newID()
	if !strings.HasPrefix(id, "myapp-") {
		t.Errorf("newID %q does not start with myapp-", id)
	}
	// "myapp-" (6 chars) + 6 hash chars = 12
	if len(id) != 12 {
		t.Errorf("newID %q length = %d, want 12", id, len(id))
	}
}

func TestEventSystem(t *testing.T) {
	store := tempDB(t)

	var received []Event
	store.On(EventCreated, func(e Event) {
		received = append(received, e)
	})

	store.emit(Event{Type: EventCreated, TaskID: "st-test", NewValue: "hello"})
	store.emit(Event{Type: EventUpdated, TaskID: "st-test"}) // different type, should not trigger

	if len(received) != 1 {
		t.Fatalf("received %d events, want 1", len(received))
	}
	if received[0].TaskID != "st-test" {
		t.Errorf("taskID = %q, want st-test", received[0].TaskID)
	}

	store.Off(EventCreated)
	store.emit(Event{Type: EventCreated, TaskID: "st-test2"})
	if len(received) != 1 {
		t.Errorf("after Off, received %d events, want 1", len(received))
	}
}
