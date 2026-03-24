package slate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultNotionConfig(t *testing.T) {
	cfg := DefaultNotionConfig()
	if cfg.RateLimit <= 0 {
		t.Error("rate limit should be positive")
	}
	if cfg.PropertyMap.Title == "" {
		t.Error("title mapping should not be empty")
	}
	if cfg.PropertyMap.Status == "" {
		t.Error("status mapping should not be empty")
	}
	if len(cfg.StatusMap) == 0 {
		t.Error("status map should not be empty")
	}
	if len(cfg.PriorityMap) == 0 {
		t.Error("priority map should not be empty")
	}
	if !cfg.AutoCreateProperties {
		t.Error("auto_create_properties should default to true")
	}
}

func TestNotionConfig_SaveAndLoad(t *testing.T) {
	home := t.TempDir()
	cfg := DefaultNotionConfig()
	cfg.Token = "test-token"
	cfg.DatabaseID = "test-db-id"

	if err := SaveNotionConfig(home, &cfg); err != nil {
		t.Fatalf("SaveNotionConfig: %v", err)
	}

	// Verify file permissions.
	info, err := os.Stat(NotionConfigPath(home))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}

	loaded, err := LoadNotionConfig(home)
	if err != nil {
		t.Fatalf("LoadNotionConfig: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded config should not be nil")
	}
	if loaded.Token != "test-token" {
		t.Errorf("token = %q, want test-token", loaded.Token)
	}
	if loaded.DatabaseID != "test-db-id" {
		t.Errorf("database_id = %q, want test-db-id", loaded.DatabaseID)
	}
	if loaded.PropertyMap.Title != "Task name" {
		t.Errorf("title = %q, want Task name", loaded.PropertyMap.Title)
	}
}

func TestNotionConfig_LoadMissing(t *testing.T) {
	home := t.TempDir()
	cfg, err := LoadNotionConfig(home)
	if err != nil {
		t.Fatalf("LoadNotionConfig: %v", err)
	}
	if cfg != nil {
		t.Error("config should be nil when file doesn't exist")
	}
}

func TestNotionConfig_Delete(t *testing.T) {
	home := t.TempDir()
	cfg := DefaultNotionConfig()
	cfg.Token = "test"
	cfg.DatabaseID = "test"

	if err := SaveNotionConfig(home, &cfg); err != nil {
		t.Fatalf("SaveNotionConfig: %v", err)
	}

	if err := DeleteNotionConfig(home); err != nil {
		t.Fatalf("DeleteNotionConfig: %v", err)
	}

	if _, err := os.Stat(NotionConfigPath(home)); !os.IsNotExist(err) {
		t.Error("config file should be deleted")
	}

	// Deleting again should not error.
	if err := DeleteNotionConfig(home); err != nil {
		t.Errorf("delete again: %v", err)
	}
}

func TestNotionConfig_DeleteMissing(t *testing.T) {
	home := t.TempDir()
	if err := DeleteNotionConfig(home); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}

func TestNotionConfig_LoadDefaults(t *testing.T) {
	home := t.TempDir()
	// Write minimal config without status_map or priority_map.
	content := []byte("token: tok\ndatabase_id: dbid\n")
	if err := os.WriteFile(filepath.Join(home, "notion.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadNotionConfig(home)
	if err != nil {
		t.Fatalf("LoadNotionConfig: %v", err)
	}
	if cfg.Token != "tok" {
		t.Errorf("token = %q", cfg.Token)
	}
	// Defaults should be applied.
	if len(cfg.StatusMap) == 0 {
		t.Error("status map defaults should be applied")
	}
	if len(cfg.PriorityMap) == 0 {
		t.Error("priority map defaults should be applied")
	}
	if cfg.RateLimit <= 0 {
		t.Error("rate limit default should be applied")
	}
}

func TestNotionConfig_StatusMapping(t *testing.T) {
	cfg := DefaultNotionConfig()

	// Push: slate → notion
	if got := cfg.StatusToNotion("open"); got != "Todo" {
		t.Errorf("StatusToNotion(open) = %q, want Todo", got)
	}
	if got := cfg.StatusToNotion("in_progress"); got != "In Progress" {
		t.Errorf("StatusToNotion(in_progress) = %q, want In Progress", got)
	}
	// Unmapped status returns raw.
	if got := cfg.StatusToNotion("deferred"); got != "deferred" {
		t.Errorf("StatusToNotion(deferred) = %q, want deferred (raw)", got)
	}

	// Pull: notion → slate
	if got := cfg.StatusFromNotion("Todo"); got != "open" {
		t.Errorf("StatusFromNotion(Todo) = %q, want open", got)
	}
	if got := cfg.StatusFromNotion("Done"); got != "closed" {
		t.Errorf("StatusFromNotion(Done) = %q, want closed", got)
	}
	// Unknown returns empty.
	if got := cfg.StatusFromNotion("Unknown"); got != "" {
		t.Errorf("StatusFromNotion(Unknown) = %q, want empty", got)
	}
}

func TestNotionConfig_StatusMapping_ManyToOne(t *testing.T) {
	cfg := DefaultNotionConfig()
	// Add many-to-one: multiple Notion statuses → one Slate status.
	cfg.StatusMap["in_progress"] = []string{"In Progress", "In Review", "On QA"}

	// All map back to in_progress.
	for _, ns := range []string{"In Progress", "In Review", "On QA"} {
		if got := cfg.StatusFromNotion(ns); got != "in_progress" {
			t.Errorf("StatusFromNotion(%s) = %q, want in_progress", ns, got)
		}
	}
	// Push uses first value.
	if got := cfg.StatusToNotion("in_progress"); got != "In Progress" {
		t.Errorf("StatusToNotion(in_progress) = %q, want In Progress (first)", got)
	}
}

func TestNotionConfig_PriorityMapping(t *testing.T) {
	cfg := DefaultNotionConfig()

	// Push.
	if got := cfg.PriorityToNotion(0); got != "High" {
		t.Errorf("PriorityToNotion(0) = %q, want High", got)
	}
	if got := cfg.PriorityToNotion(2); got != "Medium" {
		t.Errorf("PriorityToNotion(2) = %q, want Medium", got)
	}
	if got := cfg.PriorityToNotion(4); got != "Low" {
		t.Errorf("PriorityToNotion(4) = %q, want Low", got)
	}
	// Unknown priority → Medium fallback.
	if got := cfg.PriorityToNotion(99); got != "Medium" {
		t.Errorf("PriorityToNotion(99) = %q, want Medium (fallback)", got)
	}

	// Pull.
	if got := cfg.PriorityFromNotion("High"); got != 1 {
		t.Errorf("PriorityFromNotion(High) = %d, want 1", got)
	}
	if got := cfg.PriorityFromNotion("Low"); got != 3 {
		t.Errorf("PriorityFromNotion(Low) = %d, want 3", got)
	}
	// Unknown → P2 fallback.
	if got := cfg.PriorityFromNotion("Unknown"); got != 2 {
		t.Errorf("PriorityFromNotion(Unknown) = %d, want 2 (fallback)", got)
	}
}

func TestNotionConfigPath(t *testing.T) {
	got := NotionConfigPath("/home/user/.slate")
	want := "/home/user/.slate/notion.yaml"
	if got != want {
		t.Errorf("NotionConfigPath = %q, want %q", got, want)
	}
}
