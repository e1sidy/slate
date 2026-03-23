package slate

import "testing"

func TestHookMatchesFilter_NoFilter(t *testing.T) {
	h := HookDef{Command: "echo test"}
	e := Event{Type: EventClosed, NewValue: "closed"}
	if !hookMatchesFilter(h, e) {
		t.Error("no filter should always match")
	}
}

func TestHookMatchesFilter_StatusMatch(t *testing.T) {
	h := HookDef{
		Command: "echo test",
		Filter:  map[string]string{"new_status": "closed"},
	}

	match := Event{Type: EventClosed, NewValue: "closed"}
	noMatch := Event{Type: EventStatusChanged, NewValue: "open"}

	if !hookMatchesFilter(h, match) {
		t.Error("should match closed")
	}
	if hookMatchesFilter(h, noMatch) {
		t.Error("should not match open")
	}
}

func TestExpandHookVars(t *testing.T) {
	e := Event{
		TaskID:   "st-ab12",
		OldValue: "open",
		NewValue: "closed",
		Actor:    "agent-1",
		Field:    "status",
	}

	result := expandHookVars("task {id} changed {field} from {old} to {new} by {actor}", e)
	expected := "task st-ab12 changed status from open to closed by agent-1"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRunHooks_NilConfig(t *testing.T) {
	// Should not panic with nil config.
	RunHooks(nil, Event{Type: EventCreated})
}
