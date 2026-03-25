package slate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// NotionConfig holds the Notion integration configuration.
// Stored in ~/.slate/notion.yaml (separate from slate.yaml for security).
type NotionConfig struct {
	Token      string        `yaml:"token"`
	DatabaseID string        `yaml:"database_id"`
	RateLimit  time.Duration `yaml:"rate_limit"`

	// PropertyMap maps Slate field names to Notion property names.
	// Empty string means auto-create the property on first sync.
	PropertyMap PropertyMap `yaml:"property_map"`

	// DepMap maps Slate dependency types to Notion relation property names.
	DepMap map[string]string `yaml:"dep_map"`

	// StatusMap maps Slate status → list of Notion status option names.
	// First value is used for push; all values are accepted for pull.
	StatusMap map[string][]string `yaml:"status_map"`

	// PriorityMap maps Slate priority (int) → Notion select option name (for push).
	PriorityMap map[int]string `yaml:"priority_map"`

	// PriorityReverse maps Notion select option name → Slate priority (for pull).
	PriorityReverse map[string]int `yaml:"priority_reverse"`

	// SyncFilter is the default filter for selective sync (e.g., "type:epic").
	SyncFilter string `yaml:"sync_filter"`

	// DashboardPageID is the Notion page ID for the metrics dashboard.
	DashboardPageID string `yaml:"dashboard_page_id"`

	// AutoCreateProperties controls whether to auto-create missing Notion properties.
	AutoCreateProperties bool `yaml:"auto_create_properties"`

	// UserID is the Notion user ID to filter syncs by.
	// When set, only pages assigned to this user are pulled.
	// Find your ID via `slate notion status` after connecting.
	UserID string `yaml:"user_id"`

	// SprintProperty is the Notion relation property name for sprints (e.g., "Sprint").
	SprintProperty string `yaml:"sprint_property,omitempty"`

	// SprintDatabaseID is the Notion database ID for the sprints database.
	// Auto-detected from the Sprint relation property if not set.
	SprintDatabaseID string `yaml:"sprint_database_id,omitempty"`

	// SprintID is the Notion page ID of the current sprint to filter by.
	// When set, only tasks linked to this sprint are pulled.
	// Use "auto" to auto-detect the current sprint from the sprints database.
	SprintID string `yaml:"sprint_id,omitempty"`
}

// PropertyMap maps Slate field names to Notion property names.
type PropertyMap struct {
	Title       string `yaml:"title"`
	Status      string `yaml:"status"`
	Priority    string `yaml:"priority"`
	Assignee    string `yaml:"assignee"`
	Labels      string `yaml:"labels"`
	DueAt       string `yaml:"due_at"`
	ParentID    string `yaml:"parent_id"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Progress    string `yaml:"progress"`
	PRLinks     string `yaml:"pr_links"` // custom attr key → Notion URL property
}

// DefaultNotionConfig returns a NotionConfig with sensible defaults.
// These defaults match the design doc field mapping for a fresh Notion database.
func DefaultNotionConfig() NotionConfig {
	return NotionConfig{
		RateLimit: 334 * time.Millisecond, // ~3 req/sec Notion limit
		PropertyMap: PropertyMap{
			Title:       "Task name",
			Status:      "Status",
			Priority:    "Priority",
			Assignee:    "Assignee",
			Labels:      "Tags",
			DueAt:       "Due Date",
			ParentID:    "Parent-task",
			Description: "", // page body by default
			Type:        "", // auto-create if enabled
			Progress:    "", // auto-create if enabled
		},
		DepMap: map[string]string{
			"blocks":     "Blocked by",
			"relates_to": "Related to",
		},
		StatusMap: map[string][]string{
			"open":        {"Todo"},
			"in_progress": {"In Progress"},
			"blocked":     {"Blocked"},
			"closed":      {"Done"},
			"cancelled":   {"Cancelled"},
			"deferred":    {},
		},
		PriorityMap: map[int]string{
			0: "High",
			1: "High",
			2: "Medium",
			3: "Low",
			4: "Low",
		},
		PriorityReverse: map[string]int{
			"High":   1,
			"Medium": 2,
			"Low":    3,
		},
		AutoCreateProperties: true,
	}
}

// NotionConfigPath returns the path to notion.yaml in the given home directory.
func NotionConfigPath(home string) string {
	return filepath.Join(home, "notion.yaml")
}

// LoadNotionConfig reads and parses the notion.yaml configuration file.
// Returns nil, nil if the file doesn't exist (not connected).
func LoadNotionConfig(home string) (*NotionConfig, error) {
	path := NotionConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read notion config: %w", err)
	}

	var cfg NotionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse notion config: %w", err)
	}

	// Apply defaults for missing fields.
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 334 * time.Millisecond
	}
	if cfg.StatusMap == nil {
		cfg.StatusMap = DefaultNotionConfig().StatusMap
	}
	if cfg.PriorityMap == nil {
		cfg.PriorityMap = DefaultNotionConfig().PriorityMap
	}
	if cfg.PriorityReverse == nil {
		cfg.PriorityReverse = DefaultNotionConfig().PriorityReverse
	}
	if cfg.DepMap == nil {
		cfg.DepMap = DefaultNotionConfig().DepMap
	}

	return &cfg, nil
}

// SaveNotionConfig writes the notion.yaml configuration file with 0600 permissions.
func SaveNotionConfig(home string, cfg *NotionConfig) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("create home directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal notion config: %w", err)
	}

	path := NotionConfigPath(home)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write notion config: %w", err)
	}
	return nil
}

// DeleteNotionConfig removes the notion.yaml configuration file.
func DeleteNotionConfig(home string) error {
	path := NotionConfigPath(home)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete notion config: %w", err)
	}
	return nil
}

// StatusToNotion returns the Notion status option name for a Slate status (for push).
// Returns the first mapped value, or the raw status string if no mapping exists.
func (c *NotionConfig) StatusToNotion(slateStatus string) string {
	if vals, ok := c.StatusMap[slateStatus]; ok && len(vals) > 0 {
		return vals[0]
	}
	return slateStatus
}

// StatusFromNotion returns the Slate status for a Notion status option name (for pull).
// Searches all status_map values for a match. Returns empty string if no mapping found.
func (c *NotionConfig) StatusFromNotion(notionStatus string) string {
	for slateStatus, notionValues := range c.StatusMap {
		for _, v := range notionValues {
			if v == notionStatus {
				return slateStatus
			}
		}
	}
	return ""
}

// PriorityToNotion returns the Notion priority option name for a Slate priority (for push).
func (c *NotionConfig) PriorityToNotion(slatePriority int) string {
	if name, ok := c.PriorityMap[slatePriority]; ok {
		return name
	}
	return "Medium" // fallback
}

// PriorityFromNotion returns the Slate priority for a Notion priority option name (for pull).
func (c *NotionConfig) PriorityFromNotion(notionPriority string) int {
	if p, ok := c.PriorityReverse[notionPriority]; ok {
		return p
	}
	return 2 // fallback to P2 (Medium)
}
