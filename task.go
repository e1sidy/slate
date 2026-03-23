package slate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Create inserts a new task and returns it.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Task, error) {
	if p.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if p.Type == "" {
		p.Type = TypeTask
	}
	if !p.Type.IsValid() {
		return nil, fmt.Errorf("invalid task type: %s", p.Type)
	}
	if !p.Priority.IsValid() {
		return nil, fmt.Errorf("invalid priority: %d", p.Priority)
	}

	// Verify parent exists if specified.
	if err := s.validateTaskExists(p.ParentID); err != nil {
		return nil, err
	}

	now := timeNowUTC()

	// Generate ID: subtasks get ladder notation (parent.1, parent.2, ...)
	var id string
	if p.ParentID != "" {
		var count int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE parent_id = ?", p.ParentID).Scan(&count); err != nil {
			return nil, fmt.Errorf("count children: %w", err)
		}
		id = fmt.Sprintf("%s.%d", p.ParentID, count+1)
	} else {
		id = s.newID()
	}

	task := &Task{
		ID:          id,
		ParentID:    p.ParentID,
		Title:       p.Title,
		Description: p.Description,
		Status:      StatusOpen,
		Priority:    p.Priority,
		Assignee:    p.Assignee,
		Type:        p.Type,
		Labels:      p.Labels,
		Notes:       p.Notes,
		Estimate:    p.Estimate,
		DueAt:       p.DueAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   p.CreatedBy,
		Metadata:    p.Metadata,
	}

	parentID := sql.NullString{String: p.ParentID, Valid: p.ParentID != ""}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, parent_id, title, description, status, priority, assignee,
		  task_type, labels, notes, estimate, due_at, created_at, updated_at, created_by, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, parentID, p.Title, p.Description, string(StatusOpen), int(p.Priority), p.Assignee,
		string(p.Type), labelsToJSON(p.Labels), p.Notes, p.Estimate,
		timeToStr(p.DueAt), now.Format(timeFormat), now.Format(timeFormat),
		p.CreatedBy, p.Metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	s.recordEvent(id, EventCreated, p.CreatedBy, "", "", p.Title)
	return task, nil
}

// Get retrieves a single task by ID.
func (s *Store) Get(ctx context.Context, id string) (*Task, error) {
	return s.scanTask(s.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id,
	))
}

// GetFull retrieves a task with its custom attributes populated.
func (s *Store) GetFull(ctx context.Context, id string) (*Task, error) {
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	attrs, err := s.Attrs(ctx, id)
	if err != nil {
		return task, nil
	}
	if len(attrs) > 0 {
		task.Attrs = make(map[string]string, len(attrs))
		for _, a := range attrs {
			task.Attrs[a.Key] = a.Value
		}
	}
	return task, nil
}

// Update modifies fields of an existing task.
func (s *Store) Update(ctx context.Context, id string, p UpdateParams, actor string) (*Task, error) {
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	sets := []string{}
	args := []any{}

	if p.Title != nil && *p.Title != task.Title {
		s.recordEvent(id, EventUpdated, actor, "title", task.Title, *p.Title)
		sets = append(sets, "title = ?")
		args = append(args, *p.Title)
	}
	if p.Description != nil && *p.Description != task.Description {
		s.recordEvent(id, EventUpdated, actor, "description", task.Description, *p.Description)
		sets = append(sets, "description = ?")
		args = append(args, *p.Description)
	}
	if p.Priority != nil && *p.Priority != task.Priority {
		s.recordEvent(id, EventUpdated, actor, "priority", fmt.Sprintf("%d", task.Priority), fmt.Sprintf("%d", *p.Priority))
		sets = append(sets, "priority = ?")
		args = append(args, int(*p.Priority))
	}
	if p.Assignee != nil && *p.Assignee != task.Assignee {
		s.recordEvent(id, EventAssigned, actor, "assignee", task.Assignee, *p.Assignee)
		sets = append(sets, "assignee = ?")
		args = append(args, *p.Assignee)
	}
	if p.Labels != nil {
		s.recordEvent(id, EventUpdated, actor, "labels", labelsToJSON(task.Labels), labelsToJSON(*p.Labels))
		sets = append(sets, "labels = ?")
		args = append(args, labelsToJSON(*p.Labels))
	}
	if p.Orphan && task.ParentID != "" {
		s.recordEvent(id, EventUpdated, actor, "parent_id", task.ParentID, "")
		sets = append(sets, "parent_id = NULL")
	} else if p.ParentID != nil && *p.ParentID != task.ParentID {
		if *p.ParentID == "" {
			return nil, fmt.Errorf("parent ID cannot be empty, use Orphan to remove parent")
		}
		if *p.ParentID == id {
			return nil, fmt.Errorf("task cannot be its own parent")
		}
		if err := s.validateTaskExists(*p.ParentID); err != nil {
			return nil, err
		}
		s.recordEvent(id, EventUpdated, actor, "parent_id", task.ParentID, *p.ParentID)
		sets = append(sets, "parent_id = ?")
		args = append(args, *p.ParentID)
	}
	if p.Notes != nil && *p.Notes != task.Notes {
		s.recordEvent(id, EventUpdated, actor, "notes", task.Notes, *p.Notes)
		sets = append(sets, "notes = ?")
		args = append(args, *p.Notes)
	}
	if p.Estimate != nil && *p.Estimate != task.Estimate {
		s.recordEvent(id, EventUpdated, actor, "estimate", fmt.Sprintf("%d", task.Estimate), fmt.Sprintf("%d", *p.Estimate))
		sets = append(sets, "estimate = ?")
		args = append(args, *p.Estimate)
	}
	if p.DueAt != nil {
		sets = append(sets, "due_at = ?")
		args = append(args, timeToStr(p.DueAt))
	}
	if p.Metadata != nil && *p.Metadata != task.Metadata {
		sets = append(sets, "metadata = ?")
		args = append(args, *p.Metadata)
	}

	if len(sets) == 0 {
		return task, nil // nothing to update
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, timeNowUTC().Format(timeFormat))
	args = append(args, id)

	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(sets, ", "))
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	return s.Get(ctx, id)
}

// UpdateStatus changes a task's status.
func (s *Store) UpdateStatus(ctx context.Context, id string, status Status, actor string) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if task.Status == status {
		return nil
	}

	now := timeNowUTC()
	_, err = s.db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?",
		string(status), now.Format(timeFormat), id,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	s.recordEvent(id, EventStatusChanged, actor, "status", string(task.Status), string(status))

	if status == StatusInProgress {
		s.autoProgressParent(task.ParentID, actor)
	}

	return nil
}

// CloseTask marks a task as closed. Uses a transaction to ensure atomicity
// of the close + auto-unblock sequence.
func (s *Store) CloseTask(ctx context.Context, id string, reason string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if task.Status.IsTerminal() {
		return nil
	}

	// Reject if any child is not terminal.
	children, err := s.Children(ctx, id)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	for _, c := range children {
		if !c.Status.IsTerminal() {
			return fmt.Errorf("cannot close %s: child %s (%s) is %s — close or cancel all children first", id, c.ID, c.Title, c.Status)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := timeNowUTC()
	_, err = tx.ExecContext(ctx,
		"UPDATE tasks SET status = ?, closed_at = ?, close_reason = ?, updated_at = ? WHERE id = ?",
		string(StatusClosed), now.Format(timeFormat), reason, now.Format(timeFormat), id,
	)
	if err != nil {
		return fmt.Errorf("close task: %w", err)
	}

	s.recordEventTx(tx, id, EventClosed, actor, "status", string(task.Status), string(StatusClosed))

	// Auto-unblock dependents within the transaction.
	if err := s.autoUnblockTx(ctx, tx, id, actor); err != nil {
		return fmt.Errorf("auto-unblock after closing %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit close: %w", err)
	}

	// Emit events after commit (listeners see consistent state).
	s.emit(Event{TaskID: id, Type: EventClosed, Actor: actor, Field: "status",
		OldValue: string(task.Status), NewValue: string(StatusClosed), Timestamp: now})

	return nil
}

// CancelTask sets a task to cancelled. Uses a transaction for atomicity.
// Cancelling a parent cascades to all non-terminal children.
func (s *Store) CancelTask(ctx context.Context, id string, reason string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if task.Status == StatusCancelled {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.cancelTaskTx(ctx, tx, id, reason, actor); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cancel: %w", err)
	}

	return nil
}

// cancelTaskTx recursively cancels a task and its children within a transaction.
func (s *Store) cancelTaskTx(ctx context.Context, tx *sql.Tx, id string, reason string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if task.Status == StatusCancelled {
		return nil
	}

	now := timeNowUTC()
	_, err = tx.ExecContext(ctx,
		"UPDATE tasks SET status = ?, closed_at = ?, close_reason = ?, updated_at = ? WHERE id = ?",
		string(StatusCancelled), now.Format(timeFormat), reason, now.Format(timeFormat), id,
	)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}

	s.recordEventTx(tx, id, EventStatusChanged, actor, "status", string(task.Status), string(StatusCancelled))

	if err := s.autoUnblockTx(ctx, tx, id, actor); err != nil {
		return fmt.Errorf("auto-unblock after cancelling %s: %w", id, err)
	}

	// Cascade cancel to non-terminal children.
	children, err := s.Children(ctx, id)
	if err != nil {
		return fmt.Errorf("cascade cancel children: %w", err)
	}
	for _, c := range children {
		if !c.Status.IsTerminal() {
			if err := s.cancelTaskTx(ctx, tx, c.ID, "parent cancelled", actor); err != nil {
				return fmt.Errorf("cascade cancel %s: %w", c.ID, err)
			}
		}
	}

	return nil
}

// Reopen sets a closed or cancelled task back to open.
func (s *Store) Reopen(ctx context.Context, id string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.IsTerminal() {
		return fmt.Errorf("task %s is not closed or cancelled (status: %s)", id, task.Status)
	}

	now := timeNowUTC()
	_, err = s.db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, closed_at = NULL, close_reason = '', updated_at = ? WHERE id = ?",
		string(StatusOpen), now.Format(timeFormat), id,
	)
	if err != nil {
		return fmt.Errorf("reopen task: %w", err)
	}

	s.recordEvent(id, EventStatusChanged, actor, "status", string(task.Status), string(StatusOpen))
	return nil
}

// DeleteTask permanently removes a task and its children.
// Comments, dependencies, and attributes are removed via CASCADE.
func (s *Store) DeleteTask(ctx context.Context, id string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	// Recursively delete children first (depth-first).
	children, err := s.Children(ctx, id)
	if err != nil {
		return fmt.Errorf("delete task: list children: %w", err)
	}
	for _, child := range children {
		if err := s.DeleteTask(ctx, child.ID, actor); err != nil {
			return fmt.Errorf("delete child %s: %w", child.ID, err)
		}
	}

	// Delete events first (no CASCADE on events FK).
	_, err = s.db.ExecContext(ctx, "DELETE FROM events WHERE task_id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task events: %w", err)
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	s.emit(Event{
		TaskID:    id,
		Type:      EventDeleted,
		Actor:     actor,
		Field:     "title",
		OldValue:  task.Title,
		Timestamp: timeNowUTC(),
	})
	return nil
}

// Claim atomically sets assignee and status to in_progress.
// Returns ErrAlreadyClaimed if another agent has already claimed the task.
func (s *Store) Claim(ctx context.Context, id string, assignee string) (*ClaimResult, error) {
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status.IsTerminal() {
		return nil, fmt.Errorf("cannot claim %s task %s", task.Status, id)
	}

	now := timeNowUTC()

	// Atomic claim: only succeeds if task is not already claimed by someone else.
	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET assignee = ?, status = ?, updated_at = ?
		 WHERE id = ? AND (assignee = '' OR assignee IS NULL OR assignee = ?)`,
		assignee, string(StatusInProgress), now.Format(timeFormat), id, assignee,
	)
	if err != nil {
		return nil, fmt.Errorf("claim task: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("claim task rows: %w", err)
	}
	if rows == 0 {
		return nil, ErrAlreadyClaimed
	}

	if task.Assignee != assignee {
		s.recordEvent(id, EventAssigned, assignee, "assignee", task.Assignee, assignee)
	}
	if task.Status != StatusInProgress {
		s.recordEvent(id, EventStatusChanged, assignee, "status", string(task.Status), string(StatusInProgress))
	}

	result := &ClaimResult{}

	if s.autoProgressParent(task.ParentID, assignee) {
		result.ParentProgressed = true
		result.ParentID = task.ParentID
	}

	return result, nil
}

// ReleaseClaim removes the assignee and sets status back to open.
func (s *Store) ReleaseClaim(ctx context.Context, id string, actor string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if task.Status != StatusInProgress {
		return fmt.Errorf("task %s is not in_progress (status: %s)", id, task.Status)
	}

	now := timeNowUTC()
	_, err = s.db.ExecContext(ctx,
		"UPDATE tasks SET assignee = '', status = ?, updated_at = ? WHERE id = ?",
		string(StatusOpen), now.Format(timeFormat), id,
	)
	if err != nil {
		return fmt.Errorf("release claim: %w", err)
	}

	s.recordEvent(id, EventAssigned, actor, "assignee", task.Assignee, "")
	s.recordEvent(id, EventStatusChanged, actor, "status", string(StatusInProgress), string(StatusOpen))
	return nil
}

// ExpireLeases auto-releases stale claims where the task hasn't been updated
// within the lease timeout period. Returns the number of released tasks.
func (s *Store) ExpireLeases(ctx context.Context) (int, error) {
	cutoff := timeNowUTC().Add(-s.leaseTimeout)

	res, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET assignee = '', status = 'open', updated_at = ?
		 WHERE status = 'in_progress' AND assignee != '' AND updated_at < ?`,
		timeNowUTC().Format(timeFormat), cutoff.Format(timeFormat),
	)
	if err != nil {
		return 0, fmt.Errorf("expire leases: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("expire leases rows: %w", err)
	}
	return int(n), nil
}

// autoUnblockTx checks dependents of the given task within a transaction.
// If a dependent is blocked and all its blockers are terminal, transitions it to open.
func (s *Store) autoUnblockTx(ctx context.Context, tx *sql.Tx, closedID string, actor string) error {
	rows, err := s.db.QueryContext(ctx,
		"SELECT from_id FROM dependencies WHERE to_id = ? AND dep_type = 'blocks'", closedID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var dependentIDs []string
	for rows.Next() {
		var fromID string
		if err := rows.Scan(&fromID); err != nil {
			return err
		}
		dependentIDs = append(dependentIDs, fromID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, depID := range dependentIDs {
		task, err := s.Get(ctx, depID)
		if err != nil || task.Status != StatusBlocked {
			continue
		}

		// Check if all blockers are terminal.
		blockerRows, err := s.db.QueryContext(ctx,
			`SELECT t.status FROM dependencies d JOIN tasks t ON d.to_id = t.id
			 WHERE d.from_id = ? AND d.dep_type = 'blocks'`, depID)
		if err != nil {
			continue
		}

		allResolved := true
		for blockerRows.Next() {
			var status Status
			if err := blockerRows.Scan(&status); err != nil {
				allResolved = false
				break
			}
			if !status.IsTerminal() {
				allResolved = false
				break
			}
		}
		blockerRows.Close()

		if allResolved {
			now := timeNowUTC()
			_, err := tx.ExecContext(ctx,
				"UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?",
				string(StatusOpen), now.Format(timeFormat), depID,
			)
			if err != nil {
				return fmt.Errorf("unblock %s: %w", depID, err)
			}
			s.recordEventTx(tx, depID, EventStatusChanged, actor, "status", string(StatusBlocked), string(StatusOpen))
		}
	}
	return nil
}

// autoProgressParent moves a parent from open to in_progress when a child becomes active.
func (s *Store) autoProgressParent(parentID, actor string) bool {
	if parentID == "" {
		return false
	}
	parent, err := s.Get(context.Background(), parentID)
	if err != nil || parent.Status != StatusOpen {
		return false
	}
	now := timeNowUTC()
	_, err = s.db.Exec(
		"UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?",
		string(StatusInProgress), now.Format(timeFormat), parent.ID,
	)
	if err != nil {
		return false
	}
	s.recordEvent(parent.ID, EventStatusChanged, actor, "status", string(StatusOpen), string(StatusInProgress))
	return true
}
