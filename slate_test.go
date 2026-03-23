package slate

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPriorityString(t *testing.T) {
	tests := []struct {
		p    Priority
		want string
	}{
		{P0, "P0 (critical)"},
		{P1, "P1 (high)"},
		{P2, "P2 (medium)"},
		{P3, "P3 (low)"},
		{P4, "P4 (backlog)"},
		{Priority(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Priority(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestDepTypeIsValid(t *testing.T) {
	if !Blocks.IsValid() {
		t.Error("blocks should be valid")
	}
	if DepType("invalid").IsValid() {
		t.Error("invalid should not be valid")
	}
}

func TestAttrTypeIsValid(t *testing.T) {
	if !AttrString.IsValid() {
		t.Error("string should be valid")
	}
	if AttrType("invalid").IsValid() {
		t.Error("invalid should not be valid")
	}
}

func TestAttributeHelpers(t *testing.T) {
	a := &Attribute{Value: "true", Type: AttrBoolean}
	if !a.BoolValue() {
		t.Error("BoolValue should be true")
	}

	a2 := &Attribute{Value: "false"}
	if a2.BoolValue() {
		t.Error("BoolValue should be false")
	}

	a3 := &Attribute{Value: "hello"}
	if a3.StringValue() != "hello" {
		t.Error("StringValue mismatch")
	}

	a4 := &Attribute{Value: `{"key":"val"}`}
	m, err := a4.ObjectValue()
	if err != nil {
		t.Fatal(err)
	}
	if m["key"] != "val" {
		t.Errorf("ObjectValue key = %v", m["key"])
	}

	a5 := &Attribute{Value: "not json"}
	_, err = a5.ObjectValue()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWithConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Prefix:       "myapp",
		HashLen:      6,
		LeaseTimeout: 5 * time.Minute,
	}
	store, err := Open(context.Background(), filepath.Join(dir, "test.db"), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if store.Prefix() != "myapp" {
		t.Errorf("prefix = %q, want myapp", store.Prefix())
	}
}

func TestWithLeaseTimeout(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "test.db"),
		WithPrefix("st"),
		WithLeaseTimeout(10*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if store.leaseTimeout != 10*time.Minute {
		t.Errorf("leaseTimeout = %v, want 10m", store.leaseTimeout)
	}
}

func TestStrPtr(t *testing.T) {
	s := strPtr("hello")
	if *s != "hello" {
		t.Errorf("strPtr = %q, want hello", *s)
	}
}

func TestLabelsFromJSON_Edge(t *testing.T) {
	// Empty string
	if labels := labelsFromJSON(""); labels != nil {
		t.Error("empty string should return nil")
	}
	// Empty array
	if labels := labelsFromJSON("[]"); labels != nil {
		t.Error("[] should return nil")
	}
	// Valid
	labels := labelsFromJSON(`["a","b"]`)
	if len(labels) != 2 {
		t.Errorf("labels count = %d, want 2", len(labels))
	}
}

func TestEnableHooks(t *testing.T) {
	store := tempDB(t)
	cfg := &Config{
		Hooks: HookConfig{
			OnCreate: []HookDef{{Command: "echo created"}},
		},
	}
	// Should not panic.
	EnableHooks(store, cfg)
}
