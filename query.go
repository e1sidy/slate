package slate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// taskColumns is the canonical SELECT column list for the tasks table.
// Every query that returns Task rows must use this constant.
const taskColumns = `id, parent_id, title, description, status, priority, assignee,
	  task_type, labels, notes, estimate, due_at, created_at, updated_at,
	  closed_at, close_reason, created_by, metadata`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// List returns tasks matching the given filters.
func (s *Store) List(ctx context.Context, p ListParams) ([]*Task, error) {
	where := []string{}
	args := []any{}

	if p.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*p.Status))
	}
	if p.Assignee != "" {
		where = append(where, "assignee = ?")
		args = append(args, p.Assignee)
	}
	if p.Priority != nil {
		where = append(where, "priority = ?")
		args = append(args, int(*p.Priority))
	}
	if p.ParentID != nil {
		if *p.ParentID == "" {
			where = append(where, "parent_id IS NULL")
		} else {
			where = append(where, "parent_id = ?")
			args = append(args, *p.ParentID)
		}
	}
	if p.Type != nil {
		where = append(where, "task_type = ?")
		args = append(args, string(*p.Type))
	}
	if p.Label != "" {
		where = append(where, "labels LIKE ?")
		args = append(args, fmt.Sprintf("%%%s%%", p.Label))
	}
	for k, v := range p.AttrFilter {
		where = append(where, "id IN (SELECT task_id FROM custom_attributes WHERE key = ? AND value = ?)")
		args = append(args, k, v)
	}
	if len(p.ExcludeStatuses) > 0 {
		placeholders := make([]string, len(p.ExcludeStatuses))
		for i, st := range p.ExcludeStatuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		where = append(where, "status NOT IN ("+strings.Join(placeholders, ",")+")")
	}

	query := `SELECT ` + taskColumns + ` FROM tasks`

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY priority ASC, created_at DESC"

	if p.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", p.Limit)
		if p.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", p.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// Search finds tasks where the title or description matches the query.
func (s *Store) Search(ctx context.Context, query string) ([]*Task, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE title LIKE ? OR description LIKE ?
		 ORDER BY priority ASC, created_at DESC`,
		pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// Children returns direct children of a task.
func (s *Store) Children(ctx context.Context, id string) ([]*Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE parent_id = ?
		 ORDER BY id ASC`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// GetTree retrieves a task and recursively populates its Children field.
func (s *Store) GetTree(ctx context.Context, id string) (*Task, error) {
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	children, err := s.Children(ctx, id)
	if err != nil {
		return task, nil
	}

	for _, child := range children {
		subtree, err := s.GetTree(ctx, child.ID)
		if err != nil {
			task.Children = append(task.Children, child)
		} else {
			task.Children = append(task.Children, subtree)
		}
	}

	return task, nil
}

// Ready returns open tasks that have no unresolved blocking dependencies.
// If parentID is non-empty, only returns ready tasks under that parent.
func (s *Store) Ready(ctx context.Context, parentID string) ([]*Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks t
		WHERE t.status = 'open'
		AND NOT EXISTS (
			SELECT 1 FROM dependencies d
			JOIN tasks blocker ON d.to_id = blocker.id
			WHERE d.from_id = t.id
			AND d.dep_type = 'blocks'
			AND blocker.status NOT IN ('closed', 'cancelled')
		)`

	args := []any{}
	if parentID != "" {
		query += " AND t.parent_id = ?"
		args = append(args, parentID)
	}

	query += " ORDER BY t.priority ASC, t.created_at ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ready tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// Blocked returns tasks that have unresolved blocking dependencies.
func (s *Store) Blocked(ctx context.Context) ([]*Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks t
		WHERE t.status = 'blocked'
		ORDER BY t.priority ASC, t.created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("blocked tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// scanTaskRow scans a single task from any row scanner (*sql.Row or *sql.Rows).
func scanTaskRow(sc rowScanner) (*Task, error) {
	var t Task
	var parentID, closedAt, dueAt sql.NullString
	var labelsJSON, createdAt, updatedAt string

	err := sc.Scan(
		&t.ID, &parentID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Assignee, &t.Type, &labelsJSON, &t.Notes, &t.Estimate, &dueAt,
		&createdAt, &updatedAt, &closedAt, &t.CloseReason, &t.CreatedBy, &t.Metadata,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("scan task: %w", err)
	}

	if parentID.Valid {
		t.ParentID = parentID.String
	}
	t.Labels = labelsFromJSON(labelsJSON)
	if dueAt.Valid {
		t.DueAt = strToTime(dueAt.String)
	}
	if closedAt.Valid {
		t.ClosedAt = strToTime(closedAt.String)
	}
	if ct := strToTime(createdAt); ct != nil {
		t.CreatedAt = *ct
	}
	if ut := strToTime(updatedAt); ut != nil {
		t.UpdatedAt = *ut
	}

	return &t, nil
}

// scanTask reads a single task from a *sql.Row.
func (s *Store) scanTask(row *sql.Row) (*Task, error) {
	return scanTaskRow(row)
}

// scanTasks reads multiple tasks from *sql.Rows.
func scanTasks(rows *sql.Rows) ([]*Task, error) {
	var tasks []*Task
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
