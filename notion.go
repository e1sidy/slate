package slate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jomei/notionapi"
)

// NotionAPI defines the Notion API surface used by Slate.
// This interface enables mock-based testing without real API calls.
type NotionAPI interface {
	GetDatabase(ctx context.Context, id notionapi.DatabaseID) (*notionapi.Database, error)
	QueryDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error)
	UpdateDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseUpdateRequest) (*notionapi.Database, error)
	CreatePage(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error)
	UpdatePage(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error)
	GetPage(ctx context.Context, id notionapi.PageID) (*notionapi.Page, error)
	GetPageComments(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error)
	CreateComment(ctx context.Context, req *notionapi.CommentCreateRequest) (*notionapi.Comment, error)
	ListUsers(ctx context.Context, pagination *notionapi.Pagination) (*notionapi.UsersListResponse, error)
	GetBlockChildren(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error)
	AppendBlockChildren(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error)
}

// notionAPIAdapter wraps a real notionapi.Client to implement NotionAPI.
type notionAPIAdapter struct {
	client *notionapi.Client
}

func (a *notionAPIAdapter) GetDatabase(ctx context.Context, id notionapi.DatabaseID) (*notionapi.Database, error) {
	return a.client.Database.Get(ctx, id)
}

func (a *notionAPIAdapter) QueryDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseQueryRequest) (*notionapi.DatabaseQueryResponse, error) {
	return a.client.Database.Query(ctx, id, req)
}

func (a *notionAPIAdapter) UpdateDatabase(ctx context.Context, id notionapi.DatabaseID, req *notionapi.DatabaseUpdateRequest) (*notionapi.Database, error) {
	return a.client.Database.Update(ctx, id, req)
}

func (a *notionAPIAdapter) CreatePage(ctx context.Context, req *notionapi.PageCreateRequest) (*notionapi.Page, error) {
	return a.client.Page.Create(ctx, req)
}

func (a *notionAPIAdapter) UpdatePage(ctx context.Context, id notionapi.PageID, req *notionapi.PageUpdateRequest) (*notionapi.Page, error) {
	return a.client.Page.Update(ctx, id, req)
}

func (a *notionAPIAdapter) GetPage(ctx context.Context, id notionapi.PageID) (*notionapi.Page, error) {
	return a.client.Page.Get(ctx, id)
}

func (a *notionAPIAdapter) GetPageComments(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.CommentQueryResponse, error) {
	return a.client.Comment.Get(ctx, id, pagination)
}

func (a *notionAPIAdapter) CreateComment(ctx context.Context, req *notionapi.CommentCreateRequest) (*notionapi.Comment, error) {
	return a.client.Comment.Create(ctx, req)
}

func (a *notionAPIAdapter) ListUsers(ctx context.Context, pagination *notionapi.Pagination) (*notionapi.UsersListResponse, error) {
	return a.client.User.List(ctx, pagination)
}

func (a *notionAPIAdapter) GetBlockChildren(ctx context.Context, id notionapi.BlockID, pagination *notionapi.Pagination) (*notionapi.GetChildrenResponse, error) {
	return a.client.Block.GetChildren(ctx, id, pagination)
}

func (a *notionAPIAdapter) AppendBlockChildren(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error) {
	return a.client.Block.AppendChildren(ctx, id, req)
}

// NotionClient wraps the Notion API with rate limiting and configuration.
type NotionClient struct {
	API    NotionAPI
	Config *NotionConfig
	Users  map[string]notionapi.User // cached: name → user (populated on Ping)
}

// NewNotionClient creates a NotionClient from a real Notion API token.
func NewNotionClient(cfg *NotionConfig) *NotionClient {
	client := notionapi.NewClient(notionapi.Token(cfg.Token))
	return &NotionClient{
		API:    &notionAPIAdapter{client: client},
		Config: cfg,
		Users:  make(map[string]notionapi.User),
	}
}

// NewNotionClientWithAPI creates a NotionClient with a custom API implementation (for testing).
func NewNotionClientWithAPI(api NotionAPI, cfg *NotionConfig) *NotionClient {
	return &NotionClient{
		API:    api,
		Config: cfg,
		Users:  make(map[string]notionapi.User),
	}
}

// rateLimit sleeps for the configured rate limit duration.
func (nc *NotionClient) rateLimit() {
	if nc.Config.RateLimit > 0 {
		time.Sleep(nc.Config.RateLimit)
	}
}

// Ping validates the token and database access.
// Also caches workspace users for assignee mapping.
func (nc *NotionClient) Ping(ctx context.Context) error {
	db, err := nc.API.GetDatabase(ctx, notionapi.DatabaseID(nc.Config.DatabaseID))
	if err != nil {
		return fmt.Errorf("ping notion database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("notion database not found: %s", nc.Config.DatabaseID)
	}

	// Cache workspace users for assignee name→ID mapping.
	nc.rateLimit()
	if err := nc.cacheUsers(ctx); err != nil {
		// Non-fatal: assignee mapping will be limited but sync still works.
		_ = err
	}

	return nil
}

// cacheUsers fetches all workspace users and caches them by name.
func (nc *NotionClient) cacheUsers(ctx context.Context) error {
	var cursor notionapi.Cursor
	for {
		var pagination *notionapi.Pagination
		if cursor != "" {
			pagination = &notionapi.Pagination{StartCursor: cursor}
		}
		resp, err := nc.API.ListUsers(ctx, pagination)
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		for _, u := range resp.Results {
			if u.Name != "" {
				nc.Users[u.Name] = u
			}
		}
		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
		nc.rateLimit()
	}
	return nil
}

// LookupUser finds a Notion user by display name (case-sensitive).
func (nc *NotionClient) LookupUser(name string) (notionapi.User, bool) {
	u, ok := nc.Users[name]
	return u, ok
}

// DetectCurrentSprint queries the sprints database and returns the page ID
// of the sprint with status "Current" or "In Progress". Returns empty string
// if no current sprint is found.
func (nc *NotionClient) DetectCurrentSprint(ctx context.Context) (string, string, error) {
	if nc.Config.SprintDatabaseID == "" {
		return "", "", fmt.Errorf("sprint_database_id not configured")
	}

	// Try status-based detection: "Current", then "In Progress".
	for _, statusName := range []string{"Current", "In Progress"} {
		nc.rateLimit()
		resp, err := nc.API.QueryDatabase(ctx, notionapi.DatabaseID(nc.Config.SprintDatabaseID), &notionapi.DatabaseQueryRequest{
			Filter: &notionapi.PropertyFilter{
				Property: "Sprint status",
				Status: &notionapi.StatusFilterCondition{
					Equals: statusName,
				},
			},
			PageSize: 1,
		})
		if err != nil {
			continue
		}
		if len(resp.Results) > 0 {
			page := &resp.Results[0]
			name := ""
			if tp, ok := page.Properties["Sprint name"]; ok {
				if titleProp, ok := tp.(*notionapi.TitleProperty); ok && len(titleProp.Title) > 0 {
					name = titleProp.Title[0].PlainText
				}
			}
			return string(page.ID), name, nil
		}
	}

	return "", "", fmt.Errorf("no current sprint found in database %s", nc.Config.SprintDatabaseID)
}

// DetectSprintDatabaseID reads the task database schema to find the Sprint
// relation property and extract the linked sprints database ID.
func (nc *NotionClient) DetectSprintDatabaseID(ctx context.Context) (string, string, error) {
	db, err := nc.API.GetDatabase(ctx, notionapi.DatabaseID(nc.Config.DatabaseID))
	if err != nil {
		return "", "", fmt.Errorf("get database: %w", err)
	}

	// Look for Sprint relation property.
	sprintPropName := nc.Config.SprintProperty
	if sprintPropName == "" {
		sprintPropName = "Sprint" // default
	}

	prop, ok := db.Properties[sprintPropName]
	if !ok {
		return "", "", fmt.Errorf("property %q not found in database", sprintPropName)
	}

	relCfg, ok := prop.(*notionapi.RelationPropertyConfig)
	if !ok {
		return "", "", fmt.Errorf("property %q is not a relation (type: %s)", sprintPropName, prop.GetType())
	}

	dbID := string(relCfg.Relation.DatabaseID)
	if dbID == "" {
		return "", "", fmt.Errorf("property %q has no linked database", sprintPropName)
	}

	return dbID, sprintPropName, nil
}

// EnsureProperties reads the Notion database schema and creates any
// missing properties that are configured in the property map.
// Only creates properties when auto_create_properties is true.
func (nc *NotionClient) EnsureProperties(ctx context.Context) (created []string, warnings []string, err error) {
	db, err := nc.API.GetDatabase(ctx, notionapi.DatabaseID(nc.Config.DatabaseID))
	if err != nil {
		return nil, nil, fmt.Errorf("get database schema: %w", err)
	}

	existing := make(map[string]notionapi.PropertyConfig)
	for name, prop := range db.Properties {
		existing[name] = prop
	}

	// Check each mapped property.
	type propCheck struct {
		slateField   string
		notionName   string
		expectedType notionapi.PropertyConfigType
	}
	checks := []propCheck{
		{"title", nc.Config.PropertyMap.Title, notionapi.PropertyConfigTypeTitle},
		{"status", nc.Config.PropertyMap.Status, notionapi.PropertyConfigStatus},
		{"priority", nc.Config.PropertyMap.Priority, notionapi.PropertyConfigTypeSelect},
		{"assignee", nc.Config.PropertyMap.Assignee, notionapi.PropertyConfigTypePeople},
		{"labels", nc.Config.PropertyMap.Labels, notionapi.PropertyConfigTypeMultiSelect},
		{"due_at", nc.Config.PropertyMap.DueAt, notionapi.PropertyConfigTypeDate},
		{"parent_id", nc.Config.PropertyMap.ParentID, notionapi.PropertyConfigTypeRelation},
		{"type", nc.Config.PropertyMap.Type, notionapi.PropertyConfigTypeSelect},
		{"progress", nc.Config.PropertyMap.Progress, notionapi.PropertyConfigTypeRichText},
	}

	propsToCreate := make(map[string]notionapi.PropertyConfig)
	for _, c := range checks {
		if c.notionName == "" {
			continue // not mapped, skip
		}
		if prop, exists := existing[c.notionName]; exists {
			// Property exists — check type compatibility.
			if prop.GetType() != c.expectedType {
				warnings = append(warnings,
					fmt.Sprintf("%s: property %q exists as %s, expected %s",
						c.slateField, c.notionName, prop.GetType(), c.expectedType))
			}
			continue
		}
		// Property doesn't exist — create if auto-create is enabled.
		if !nc.Config.AutoCreateProperties {
			warnings = append(warnings,
				fmt.Sprintf("%s: property %q not found (auto_create_properties is false)",
					c.slateField, c.notionName))
			continue
		}
		propCfg := buildPropertyConfig(c.expectedType, c.notionName)
		if propCfg != nil {
			propsToCreate[c.notionName] = propCfg
			created = append(created, c.notionName)
		}
	}

	if len(propsToCreate) > 0 {
		nc.rateLimit()
		_, err := nc.API.UpdateDatabase(ctx, notionapi.DatabaseID(nc.Config.DatabaseID), &notionapi.DatabaseUpdateRequest{
			Properties: propsToCreate,
		})
		if err != nil {
			return created, warnings, fmt.Errorf("create properties: %w", err)
		}
	}

	return created, warnings, nil
}

// buildPropertyConfig creates a Notion property configuration for the given type.
func buildPropertyConfig(propType notionapi.PropertyConfigType, name string) notionapi.PropertyConfig {
	switch propType {
	case notionapi.PropertyConfigTypeSelect:
		return notionapi.SelectPropertyConfig{
			Type:   notionapi.PropertyConfigTypeSelect,
			Select: notionapi.Select{},
		}
	case notionapi.PropertyConfigTypeMultiSelect:
		return notionapi.MultiSelectPropertyConfig{
			Type:        notionapi.PropertyConfigTypeMultiSelect,
			MultiSelect: notionapi.Select{},
		}
	case notionapi.PropertyConfigTypeRichText:
		return notionapi.RichTextPropertyConfig{
			Type: notionapi.PropertyConfigTypeRichText,
		}
	case notionapi.PropertyConfigTypeDate:
		return notionapi.DatePropertyConfig{
			Type: notionapi.PropertyConfigTypeDate,
		}
	case notionapi.PropertyConfigTypePeople:
		return notionapi.PeoplePropertyConfig{
			Type: notionapi.PropertyConfigTypePeople,
		}
	default:
		// Title, Status, Relation — cannot be created via Update API.
		return nil
	}
}

// InferMapping reads the Notion database schema and produces a NotionConfig
// with property_map auto-populated by matching property names and types.
func InferMapping(ctx context.Context, api NotionAPI, databaseID string) (*NotionConfig, error) {
	db, err := api.GetDatabase(ctx, notionapi.DatabaseID(databaseID))
	if err != nil {
		return nil, fmt.Errorf("get database: %w", err)
	}

	cfg := DefaultNotionConfig()
	cfg.DatabaseID = databaseID

	// Build a map of property name → type for matching.
	propTypes := make(map[string]notionapi.PropertyConfigType)
	for name, prop := range db.Properties {
		propTypes[name] = prop.GetType()
	}

	// Try to match known property names/types.
	type matcher struct {
		field    *string
		names    []string
		expected notionapi.PropertyConfigType
	}
	matchers := []matcher{
		{&cfg.PropertyMap.Title, []string{"Task name", "Name", "Title"}, notionapi.PropertyConfigTypeTitle},
		{&cfg.PropertyMap.Status, []string{"Status"}, notionapi.PropertyConfigStatus},
		{&cfg.PropertyMap.Priority, []string{"Priority"}, notionapi.PropertyConfigTypeSelect},
		{&cfg.PropertyMap.Assignee, []string{"Assignee", "Assign", "Owner"}, notionapi.PropertyConfigTypePeople},
		{&cfg.PropertyMap.Labels, []string{"Tags", "Labels"}, notionapi.PropertyConfigTypeMultiSelect},
		{&cfg.PropertyMap.DueAt, []string{"Due Date", "Due", "Deadline"}, notionapi.PropertyConfigTypeDate},
		{&cfg.PropertyMap.ParentID, []string{"Parent-task", "Parent", "Parent task"}, notionapi.PropertyConfigTypeRelation},
	}

	for _, m := range matchers {
		*m.field = "" // reset
		for _, name := range m.names {
			if propTypes[name] == m.expected {
				*m.field = name
				break
			}
		}
	}

	// Infer status mapping from Notion's status options.
	if statusPropName := cfg.PropertyMap.Status; statusPropName != "" {
		if prop, ok := db.Properties[statusPropName]; ok {
			if statusCfg, ok := prop.(*notionapi.StatusPropertyConfig); ok {
				cfg.StatusMap = inferStatusMap(*statusCfg)
			}
		}
	}

	// Infer priority mapping from Notion's select options.
	if priorityPropName := cfg.PropertyMap.Priority; priorityPropName != "" {
		if prop, ok := db.Properties[priorityPropName]; ok {
			if selectCfg, ok := prop.(*notionapi.SelectPropertyConfig); ok {
				cfg.PriorityMap, cfg.PriorityReverse = inferPriorityMap(*selectCfg)
			}
		}
	}

	// Infer dependency mapping.
	for name, ptype := range propTypes {
		if ptype != notionapi.PropertyConfigTypeRelation {
			continue
		}
		nameLower := name
		switch nameLower {
		case "Blocked by":
			cfg.DepMap["blocks"] = name
		case "Related to":
			cfg.DepMap["relates_to"] = name
		}
	}

	return &cfg, nil
}

// inferStatusMap tries to map Notion status options to Slate statuses.
func inferStatusMap(prop notionapi.StatusPropertyConfig) map[string][]string {
	m := map[string][]string{
		"open":        {},
		"in_progress": {},
		"blocked":     {},
		"closed":      {},
		"cancelled":   {},
		"deferred":    {},
	}

	// Build option ID → name lookup.
	optionNames := make(map[string]string) // ObjectID string → name
	for _, opt := range prop.Status.Options {
		optionNames[string(opt.ID)] = opt.Name
	}

	for _, group := range prop.Status.Groups {
		for _, optID := range group.OptionIDs {
			name := optionNames[string(optID)]
			if name == "" {
				continue
			}
			switch group.Name {
			case "To-do", "To Do":
				if name == "Blocked" || name == "RCA Pending" {
					m["blocked"] = append(m["blocked"], name)
				} else {
					m["open"] = append(m["open"], name)
				}
			case "In progress":
				m["in_progress"] = append(m["in_progress"], name)
			case "Complete":
				if name == "Cancelled" {
					m["cancelled"] = append(m["cancelled"], name)
				} else {
					m["closed"] = append(m["closed"], name)
				}
			default:
				m["open"] = append(m["open"], name)
			}
		}
	}

	return m
}

// inferPriorityMap maps Notion select options to Slate priorities.
func inferPriorityMap(prop notionapi.SelectPropertyConfig) (map[int]string, map[string]int) {
	forward := map[int]string{}
	reverse := map[string]int{}

	for _, opt := range prop.Select.Options {
		switch opt.Name {
		case "Critical", "Urgent":
			forward[0] = opt.Name
			reverse[opt.Name] = 0
		case "High":
			forward[0] = opt.Name
			forward[1] = opt.Name
			reverse[opt.Name] = 1
		case "Medium":
			forward[2] = opt.Name
			reverse[opt.Name] = 2
		case "Low":
			forward[3] = opt.Name
			forward[4] = opt.Name
			reverse[opt.Name] = 3
		}
	}

	return forward, reverse
}

// --- Notion Sync Records (stored in SQLite notion_sync table) ---

// NotionSyncRecord tracks the mapping between a Slate task and a Notion page.
type NotionSyncRecord struct {
	TaskID         string
	NotionPageID   string
	LastSyncedAt   time.Time
	SyncDirection  string // "both", "push", "pull"
	ConflictStatus string // empty, or JSON describing the conflict
}

// GetSyncRecord returns the sync record for a Slate task, or nil if not synced.
func (s *Store) GetSyncRecord(ctx context.Context, taskID string) (*NotionSyncRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT task_id, notion_page_id, last_synced_at, sync_direction, conflict_status
		 FROM notion_sync WHERE task_id = ?`, taskID)
	return scanSyncRecord(row)
}

// GetSyncRecordByPage returns the sync record for a Notion page ID, or nil if not found.
func (s *Store) GetSyncRecordByPage(ctx context.Context, pageID string) (*NotionSyncRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT task_id, notion_page_id, last_synced_at, sync_direction, conflict_status
		 FROM notion_sync WHERE notion_page_id = ?`, pageID)
	return scanSyncRecord(row)
}

func scanSyncRecord(row *sql.Row) (*NotionSyncRecord, error) {
	var r NotionSyncRecord
	var syncedAt string
	err := row.Scan(&r.TaskID, &r.NotionPageID, &syncedAt, &r.SyncDirection, &r.ConflictStatus)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan sync record: %w", err)
	}
	r.LastSyncedAt, _ = time.Parse(timeFormat, syncedAt)
	return &r, nil
}

// UpsertSyncRecord creates or updates a sync record.
func (s *Store) UpsertSyncRecord(ctx context.Context, r *NotionSyncRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notion_sync (task_id, notion_page_id, last_synced_at, sync_direction, conflict_status)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(task_id) DO UPDATE SET
		   notion_page_id = excluded.notion_page_id,
		   last_synced_at = excluded.last_synced_at,
		   sync_direction = excluded.sync_direction,
		   conflict_status = excluded.conflict_status`,
		r.TaskID, r.NotionPageID, r.LastSyncedAt.Format(timeFormat),
		r.SyncDirection, r.ConflictStatus)
	if err != nil {
		return fmt.Errorf("upsert sync record: %w", err)
	}
	return nil
}

// DeleteSyncRecord removes the sync record for a task.
func (s *Store) DeleteSyncRecord(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM notion_sync WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("delete sync record: %w", err)
	}
	return nil
}

// ListSyncRecords returns all sync records.
func (s *Store) ListSyncRecords(ctx context.Context) ([]*NotionSyncRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id, notion_page_id, last_synced_at, sync_direction, conflict_status
		 FROM notion_sync ORDER BY task_id`)
	if err != nil {
		return nil, fmt.Errorf("list sync records: %w", err)
	}
	defer rows.Close()

	var records []*NotionSyncRecord
	for rows.Next() {
		var r NotionSyncRecord
		var syncedAt string
		if err := rows.Scan(&r.TaskID, &r.NotionPageID, &syncedAt, &r.SyncDirection, &r.ConflictStatus); err != nil {
			return nil, fmt.Errorf("scan sync record: %w", err)
		}
		r.LastSyncedAt, _ = time.Parse(timeFormat, syncedAt)
		records = append(records, &r)
	}
	return records, rows.Err()
}

// ListConflicts returns sync records that have unresolved conflicts.
func (s *Store) ListConflicts(ctx context.Context) ([]*NotionSyncRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id, notion_page_id, last_synced_at, sync_direction, conflict_status
		 FROM notion_sync WHERE conflict_status != '' ORDER BY task_id`)
	if err != nil {
		return nil, fmt.Errorf("list conflicts: %w", err)
	}
	defer rows.Close()

	var records []*NotionSyncRecord
	for rows.Next() {
		var r NotionSyncRecord
		var syncedAt string
		if err := rows.Scan(&r.TaskID, &r.NotionPageID, &syncedAt, &r.SyncDirection, &r.ConflictStatus); err != nil {
			return nil, fmt.Errorf("scan conflict record: %w", err)
		}
		r.LastSyncedAt, _ = time.Parse(timeFormat, syncedAt)
		records = append(records, &r)
	}
	return records, rows.Err()
}
