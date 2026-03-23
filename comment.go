package slate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// AddComment adds a freeform text comment to a task.
func (s *Store) AddComment(ctx context.Context, taskID, author, content string) (*Comment, error) {
	if content == "" {
		return nil, fmt.Errorf("comment content is required")
	}
	if err := s.validateTaskExists(taskID); err != nil {
		return nil, err
	}

	now := timeNowUTC()
	id := uuid.New().String()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (id, task_id, author, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, taskID, author, content, now.Format(timeFormat), now.Format(timeFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	s.recordEvent(taskID, EventCommented, author, "comment", "", content)

	return &Comment{
		ID:        id,
		TaskID:    taskID,
		Author:    author,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// EditComment updates the content of an existing comment.
func (s *Store) EditComment(ctx context.Context, commentID, content string) error {
	if content == "" {
		return fmt.Errorf("comment content is required")
	}

	now := timeNowUTC()
	res, err := s.db.ExecContext(ctx,
		"UPDATE comments SET content = ?, updated_at = ? WHERE id = ?",
		content, now.Format(timeFormat), commentID,
	)
	if err != nil {
		return fmt.Errorf("edit comment: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("comment %s not found", commentID)
	}
	return nil
}

// DeleteComment removes a comment.
func (s *Store) DeleteComment(ctx context.Context, commentID string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", commentID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("comment %s not found", commentID)
	}
	return nil
}

// ListComments returns all comments for a task in creation order.
func (s *Store) ListComments(ctx context.Context, taskID string) ([]*Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, author, content, created_at, updated_at
		 FROM comments WHERE task_id = ? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		var c Comment
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.Content, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if ct := strToTime(createdAt); ct != nil {
			c.CreatedAt = *ct
		}
		if ut := strToTime(updatedAt); ut != nil {
			c.UpdatedAt = *ut
		}
		comments = append(comments, &c)
	}
	return comments, rows.Err()
}
