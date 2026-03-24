package slate

import (
	"context"
	"fmt"
)

// SearchFTS performs full-text search using FTS5.
// Returns tasks matching the query, ranked by relevance.
// Falls back to LIKE-based Search if FTS5 is not available.
func (s *Store) SearchFTS(ctx context.Context, query string) ([]*Task, error) {
	if query == "" {
		return nil, nil
	}

	// Check if FTS5 table exists.
	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tasks_fts'").Scan(&exists)
	if err != nil || exists == 0 {
		// Fall back to LIKE-based search.
		return s.Search(ctx, query)
	}

	// FTS5 query: join tasks_fts with tasks to get full task data.
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks
		 WHERE id IN (
		     SELECT task_id FROM tasks_fts WHERE tasks_fts MATCH ?
		 )
		 ORDER BY priority ASC, created_at DESC`,
		query)
	if err != nil {
		// FTS5 query failed (bad syntax?), fall back to LIKE.
		return s.Search(ctx, query)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// RebuildFTSIndex rebuilds the FTS5 index from current task data.
// Should be called after bulk imports.
func (s *Store) RebuildFTSIndex(ctx context.Context) error {
	// Check if FTS5 table exists.
	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tasks_fts'").Scan(&exists)
	if err != nil || exists == 0 {
		return nil // FTS5 not available, nothing to rebuild
	}

	// Clear and repopulate.
	_, err = s.db.ExecContext(ctx, "DELETE FROM tasks_fts")
	if err != nil {
		return fmt.Errorf("clear FTS index: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO tasks_fts(task_id, title, description, notes) SELECT id, title, description, notes FROM tasks")
	if err != nil {
		return fmt.Errorf("rebuild FTS index: %w", err)
	}

	return nil
}
