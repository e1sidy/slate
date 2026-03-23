package slate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Prefix != "st" {
		t.Errorf("prefix = %q, want st", cfg.Prefix)
	}
	if cfg.HashLen != 4 {
		t.Errorf("hashLen = %d, want 4", cfg.HashLen)
	}
	if cfg.LeaseTimeout != 30*time.Minute {
		t.Errorf("leaseTimeout = %v, want 30m", cfg.LeaseTimeout)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/slate.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "st" {
		t.Errorf("prefix = %q, want st (default)", cfg.Prefix)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slate.yaml")

	original := &Config{
		Prefix:       "myapp",
		DBPath:       filepath.Join(dir, "myapp.db"),
		HashLen:      6,
		DefaultView:  "tree",
		ShowAll:      true,
		LeaseTimeout: 15 * time.Minute,
	}

	if err := SaveConfig(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Prefix != "myapp" {
		t.Errorf("prefix = %q, want myapp", loaded.Prefix)
	}
	if loaded.HashLen != 6 {
		t.Errorf("hashLen = %d, want 6", loaded.HashLen)
	}
	if loaded.DefaultView != "tree" {
		t.Errorf("defaultView = %q, want tree", loaded.DefaultView)
	}
	if !loaded.ShowAll {
		t.Error("showAll = false, want true")
	}
}

func TestLoadConfig_InvalidHashLen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slate.yaml")

	os.WriteFile(path, []byte("hash_length: 99\n"), 0o644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HashLen != 4 {
		t.Errorf("hashLen = %d, want 4 (default for invalid)", cfg.HashLen)
	}
}

func TestLoadConfig_InvalidDefaultView(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slate.yaml")

	os.WriteFile(path, []byte("default_view: kanban\n"), 0o644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultView != "" {
		t.Errorf("defaultView = %q, want empty (reset for invalid)", cfg.DefaultView)
	}
}

func TestDefaultSlateHome_EnvOverride(t *testing.T) {
	t.Setenv("SLATE_HOME", "/tmp/custom-slate")
	home := DefaultSlateHome()
	if home != "/tmp/custom-slate" {
		t.Errorf("home = %q, want /tmp/custom-slate", home)
	}
}
