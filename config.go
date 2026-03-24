package slate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the slate.yaml configuration file.
type Config struct {
	Prefix       string        `yaml:"prefix"`
	DBPath       string        `yaml:"db_path"`
	HashLen      int           `yaml:"hash_length"`
	DefaultView  string        `yaml:"default_view"` // "list" or "tree" (default: "list")
	ShowAll      bool          `yaml:"show_all"`     // if true, include closed tasks by default
	LeaseTimeout time.Duration `yaml:"lease_timeout"` // auto-release claims after this duration (default: 30m)
	Hooks        HookConfig    `yaml:"hooks"`
}

// HookConfig maps event types to lists of hook definitions.
type HookConfig struct {
	OnStatusChange []HookDef `yaml:"on_status_change"`
	OnCreate       []HookDef `yaml:"on_create"`
	OnComment      []HookDef `yaml:"on_comment"`
	OnClose        []HookDef `yaml:"on_close"`
	OnAssign       []HookDef `yaml:"on_assign"`
}

// HookDef defines a single hook (shell command or webhook) with optional filter.
type HookDef struct {
	Command string            `yaml:"command,omitempty"`
	Webhook string            `yaml:"webhook,omitempty"`  // URL for webhook hooks
	Method  string            `yaml:"method,omitempty"`   // HTTP method (default: POST)
	Headers map[string]string `yaml:"headers,omitempty"`  // HTTP headers
	Body    string            `yaml:"body,omitempty"`     // JSON body template with {id}, {new}, etc.
	Timeout int               `yaml:"timeout,omitempty"`  // Timeout in seconds (default: 10)
	Filter  map[string]string `yaml:"filter,omitempty"`
}

// DefaultSlateHome returns the default slate home directory (~/.slate).
// Respects SLATE_HOME env var.
func DefaultSlateHome() string {
	if env := os.Getenv("SLATE_HOME"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".slate"
	}
	return filepath.Join(home, ".slate")
}

// DefaultDBPath returns the default database path.
func DefaultDBPath() string {
	return filepath.Join(DefaultSlateHome(), "slate.db")
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return filepath.Join(DefaultSlateHome(), "slate.yaml")
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Prefix:       "st",
		DBPath:       DefaultDBPath(),
		HashLen:      4,
		LeaseTimeout: 30 * time.Minute,
	}
}

// LoadConfig reads and parses a slate.yaml file.
// Returns DefaultConfig if the file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults for empty fields.
	if cfg.Prefix == "" {
		cfg.Prefix = "st"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = DefaultDBPath()
	}
	if cfg.HashLen < 3 || cfg.HashLen > 8 {
		cfg.HashLen = 4
	}
	if cfg.DefaultView != "" && cfg.DefaultView != "list" && cfg.DefaultView != "tree" {
		cfg.DefaultView = ""
	}
	if cfg.LeaseTimeout <= 0 {
		cfg.LeaseTimeout = 30 * time.Minute
	}

	return &cfg, nil
}

// SaveConfig writes the config to a YAML file.
func SaveConfig(path string, cfg *Config) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}
