package slate

import (
	"context"
	"fmt"
	"time"
)

// Events returns all events for a task, ordered by timestamp.
func (s *Store) Events(ctx context.Context, taskID string) ([]*Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, event_type, actor, field, old_value, new_value, timestamp
		 FROM events WHERE task_id = ? ORDER BY timestamp ASC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// EventsSince returns events for a task after the given time.
func (s *Store) EventsSince(ctx context.Context, taskID string, since time.Time) ([]*Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, event_type, actor, field, old_value, new_value, timestamp
		 FROM events WHERE task_id = ? AND timestamp > ?
		 ORDER BY timestamp ASC, id ASC`, taskID, since.Format(timeFormat))
	if err != nil {
		return nil, fmt.Errorf("events since: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

func scanEvents(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*Event, error) {
	var events []*Event
	for rows.Next() {
		var e Event
		var timestamp string
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Type, &e.Actor, &e.Field,
			&e.OldValue, &e.NewValue, &timestamp); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if t := strToTime(timestamp); t != nil {
			e.Timestamp = *t
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}
