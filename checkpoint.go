package slate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// AddCheckpoint creates a structured progress snapshot for a task.
func (s *Store) AddCheckpoint(ctx context.Context, taskID, author string, p CheckpointParams) (*Checkpoint, error) {
	if p.Done == "" {
		return nil, fmt.Errorf("checkpoint 'done' field is required")
	}
	if err := s.validateTaskExists(taskID); err != nil {
		return nil, err
	}

	now := timeNowUTC()
	id := uuid.New().String()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO checkpoints (id, task_id, author, done, decisions, next, blockers, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, taskID, author, p.Done, p.Decisions, p.Next, p.Blockers, now.Format(timeFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("insert checkpoint: %w", err)
	}

	// Insert file references.
	for _, f := range p.Files {
		_, err := s.db.ExecContext(ctx,
			"INSERT INTO checkpoint_files (checkpoint_id, file_path) VALUES (?, ?)",
			id, f,
		)
		if err != nil {
			return nil, fmt.Errorf("insert checkpoint file: %w", err)
		}
	}

	return &Checkpoint{
		ID:        id,
		TaskID:    taskID,
		Author:    author,
		Done:      p.Done,
		Decisions: p.Decisions,
		Next:      p.Next,
		Blockers:  p.Blockers,
		Files:     p.Files,
		CreatedAt: now,
	}, nil
}

// ListCheckpoints returns all checkpoints for a task in creation order.
func (s *Store) ListCheckpoints(ctx context.Context, taskID string) ([]*Checkpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, author, done, decisions, next, blockers, created_at
		 FROM checkpoints WHERE task_id = ? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []*Checkpoint
	for rows.Next() {
		var cp Checkpoint
		var createdAt string
		if err := rows.Scan(&cp.ID, &cp.TaskID, &cp.Author, &cp.Done,
			&cp.Decisions, &cp.Next, &cp.Blockers, &createdAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		if ct := strToTime(createdAt); ct != nil {
			cp.CreatedAt = *ct
		}
		cp.Files = s.checkpointFiles(ctx, cp.ID)
		checkpoints = append(checkpoints, &cp)
	}
	return checkpoints, rows.Err()
}

// LatestCheckpoint returns the most recent checkpoint for a task.
func (s *Store) LatestCheckpoint(ctx context.Context, taskID string) (*Checkpoint, error) {
	var cp Checkpoint
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, task_id, author, done, decisions, next, blockers, created_at
		 FROM checkpoints WHERE task_id = ? ORDER BY rowid DESC LIMIT 1`, taskID,
	).Scan(&cp.ID, &cp.TaskID, &cp.Author, &cp.Done, &cp.Decisions, &cp.Next, &cp.Blockers, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("latest checkpoint: %w", err)
	}
	if ct := strToTime(createdAt); ct != nil {
		cp.CreatedAt = *ct
	}
	cp.Files = s.checkpointFiles(ctx, cp.ID)
	return &cp, nil
}

func (s *Store) checkpointFiles(ctx context.Context, checkpointID string) []string {
	rows, err := s.db.QueryContext(ctx,
		"SELECT file_path FROM checkpoint_files WHERE checkpoint_id = ?", checkpointID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err == nil {
			files = append(files, f)
		}
	}
	return files
}
