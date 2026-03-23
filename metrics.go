package slate

import (
	"context"
	"fmt"
	"time"
)

// MetricsReport holds aggregated metrics.
type MetricsReport struct {
	TasksCreated  int           `json:"tasks_created"`
	TasksClosed   int           `json:"tasks_closed"`
	TasksCancelled int          `json:"tasks_cancelled"`
	CurrentOpen   int           `json:"current_open"`
	CurrentBlocked int          `json:"current_blocked"`
	AvgCycleTime  time.Duration `json:"avg_cycle_time"`
}

// MetricsParams controls the metrics query.
type MetricsParams struct {
	From  *time.Time
	To    *time.Time
	Actor string
}

// Metrics returns aggregated task metrics for the given time range.
func (s *Store) Metrics(ctx context.Context, p MetricsParams) (*MetricsReport, error) {
	report := &MetricsReport{}

	from := "1970-01-01T00:00:00Z"
	to := "9999-12-31T23:59:59Z"
	if p.From != nil {
		from = p.From.Format(timeFormat)
	}
	if p.To != nil {
		to = p.To.Format(timeFormat)
	}

	// Tasks created in range.
	q := "SELECT COUNT(*) FROM events WHERE event_type = 'created' AND timestamp BETWEEN ? AND ?"
	args := []any{from, to}
	if p.Actor != "" {
		q += " AND actor = ?"
		args = append(args, p.Actor)
	}
	s.db.QueryRowContext(ctx, q, args...).Scan(&report.TasksCreated)

	// Tasks closed in range.
	q = "SELECT COUNT(*) FROM events WHERE event_type = 'closed' AND timestamp BETWEEN ? AND ?"
	args = []any{from, to}
	if p.Actor != "" {
		q += " AND actor = ?"
		args = append(args, p.Actor)
	}
	s.db.QueryRowContext(ctx, q, args...).Scan(&report.TasksClosed)

	// Tasks cancelled in range.
	q = "SELECT COUNT(*) FROM events WHERE event_type = 'status_changed' AND new_value = 'cancelled' AND timestamp BETWEEN ? AND ?"
	args = []any{from, to}
	s.db.QueryRowContext(ctx, q, args...).Scan(&report.TasksCancelled)

	// Current open.
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'open'").Scan(&report.CurrentOpen)

	// Current blocked.
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'blocked'").Scan(&report.CurrentBlocked)

	// Average cycle time (created → closed).
	report.AvgCycleTime = s.avgCycleTime(ctx, from, to)

	return report, nil
}

// CycleTime returns the duration from creation to close for a specific task.
func (s *Store) CycleTime(ctx context.Context, taskID string) (time.Duration, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return 0, err
	}
	if task.ClosedAt == nil {
		return 0, fmt.Errorf("task %s is not closed", taskID)
	}
	return task.ClosedAt.Sub(task.CreatedAt), nil
}

// Throughput returns the count of tasks closed in the given time range.
func (s *Store) Throughput(ctx context.Context, from, to time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM events WHERE event_type = 'closed' AND timestamp BETWEEN ? AND ?",
		from.Format(timeFormat), to.Format(timeFormat),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("throughput: %w", err)
	}
	return count, nil
}

// Next suggests the highest-impact ready task to work on.
// Impact = number of transitive dependents that would be unblocked.
func (s *Store) Next(ctx context.Context) (*Task, error) {
	ready, err := s.Ready(ctx, "")
	if err != nil {
		return nil, err
	}
	if len(ready) == 0 {
		return nil, fmt.Errorf("no ready tasks")
	}

	var best *Task
	bestScore := -1

	for _, task := range ready {
		score := s.countTransitiveDependents(ctx, task.ID)
		if score > bestScore {
			bestScore = score
			best = task
		}
	}

	return best, nil
}

// countTransitiveDependents counts how many tasks depend (directly or transitively)
// on the given task being completed.
func (s *Store) countTransitiveDependents(ctx context.Context, taskID string) int {
	visited := make(map[string]bool)
	queue := []string{taskID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		rows, err := s.db.QueryContext(ctx,
			"SELECT from_id FROM dependencies WHERE to_id = ? AND dep_type = 'blocks'", current)
		if err != nil {
			continue
		}
		for rows.Next() {
			var depID string
			if rows.Scan(&depID) == nil && !visited[depID] {
				queue = append(queue, depID)
			}
		}
		rows.Close()
	}

	// Subtract 1 because we counted the task itself.
	count := len(visited) - 1
	if count < 0 {
		count = 0
	}
	return count
}

func (s *Store) avgCycleTime(ctx context.Context, from, to string) time.Duration {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.created_at, t.closed_at FROM tasks t
		 WHERE t.status = 'closed' AND t.closed_at != '' AND t.closed_at IS NOT NULL
		 AND t.closed_at BETWEEN ? AND ?`, from, to)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var total time.Duration
	var count int
	for rows.Next() {
		var createdStr, closedStr string
		if rows.Scan(&createdStr, &closedStr) != nil {
			continue
		}
		created := strToTime(createdStr)
		closed := strToTime(closedStr)
		if created != nil && closed != nil {
			total += closed.Sub(*created)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}
