package slate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/e1sidy/slate/internal/migrate"
)

// ArchiveResult summarizes the archive operation.
type ArchiveResult struct {
	Archived int `json:"archived"`
}

// Archive moves closed tasks older than the given date to an archive database.
// Events are kept in the main DB for metrics continuity.
// Comments, deps, attrs, and checkpoints for archived tasks are also moved.
func (s *Store) Archive(ctx context.Context, before time.Time, archivePath string) (*ArchiveResult, error) {
	// Open (or create) archive database.
	archiveDB, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive db: %w", err)
	}
	defer archiveDB.Close()

	// Run migrations on archive DB to ensure schema exists.
	if err := migrate.Run(archiveDB); err != nil {
		return nil, fmt.Errorf("migrate archive db: %w", err)
	}

	// Find closed tasks older than the cutoff.
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks
		 WHERE (status = 'closed' OR status = 'cancelled')
		   AND closed_at != '' AND closed_at < ?
		 ORDER BY id`,
		before.Format(timeFormat))
	if err != nil {
		return nil, fmt.Errorf("query tasks to archive: %w", err)
	}
	tasks, err := scanTasks(rows)
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		return &ArchiveResult{Archived: 0}, nil
	}

	// Sort tasks: children before parents to avoid FK violations on delete.
	// Tasks with a parent_id that's in the archive set must be deleted first.
	idSet := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		idSet[t.ID] = true
	}
	// Partition into children-of-archived-parents first, then the rest.
	var ordered []*Task
	var parents []*Task
	for _, t := range tasks {
		if t.ParentID != "" && idSet[t.ParentID] {
			ordered = append(ordered, t)
		} else {
			parents = append(parents, t)
		}
	}
	ordered = append(ordered, parents...)

	// Copy each task + related data to archive, then delete from main.
	for _, task := range ordered {
		if err := copyTaskToArchive(ctx, s.db, archiveDB, task.ID); err != nil {
			return nil, fmt.Errorf("archive task %s: %w", task.ID, err)
		}
		// Delete events referencing this task first (events don't have ON DELETE CASCADE).
		s.db.ExecContext(ctx, "DELETE FROM events WHERE task_id = ?", task.ID)
		// Clear parent_id references from non-archived children to avoid FK issues.
		// Use NULL instead of '' because FK constraint validates non-empty strings.
		s.db.ExecContext(ctx, "UPDATE tasks SET parent_id = NULL WHERE parent_id = ?", task.ID)
		// Delete task from main DB (cascade deletes comments, deps, attrs, checkpoints, notion_sync).
		if _, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", task.ID); err != nil {
			return nil, fmt.Errorf("delete task %s: %w", task.ID, err)
		}
	}

	return &ArchiveResult{Archived: len(tasks)}, nil
}

// Unarchive restores tasks from an archive database back to the main database.
// If taskIDs is empty, restores all archived tasks.
func (s *Store) Unarchive(ctx context.Context, archivePath string, taskIDs []string) (int, error) {
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("archive file not found: %s", archivePath)
	}

	archiveDB, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return 0, fmt.Errorf("open archive db: %w", err)
	}
	defer archiveDB.Close()

	// Get tasks to restore.
	var query string
	var args []any
	if len(taskIDs) > 0 {
		placeholders := ""
		for i, id := range taskIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, id)
		}
		query = "SELECT " + taskColumns + " FROM tasks WHERE id IN (" + placeholders + ")"
	} else {
		query = "SELECT " + taskColumns + " FROM tasks"
	}

	rows, err := archiveDB.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("query archive: %w", err)
	}
	tasks, err := scanTasks(rows)
	rows.Close()
	if err != nil {
		return 0, fmt.Errorf("scan archive: %w", err)
	}

	// Copy each task back to main DB.
	restored := 0
	for _, task := range tasks {
		if err := copyTaskToArchive(ctx, archiveDB, s.db, task.ID); err != nil {
			continue // skip tasks that fail (e.g., already exist)
		}
		// Delete from archive.
		archiveDB.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", task.ID)
		restored++
	}

	return restored, nil
}

// ListArchived lists tasks in the archive database.
func ListArchived(ctx context.Context, archivePath string) ([]*Task, error) {
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return nil, nil
	}

	archiveDB, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer archiveDB.Close()

	rows, err := archiveDB.QueryContext(ctx,
		"SELECT "+taskColumns+" FROM tasks ORDER BY closed_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query archive: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

// DefaultArchivePath returns the default archive database path.
func DefaultArchivePath() string {
	return DefaultSlateHome() + "/slate-archive.db"
}

// copyTaskToArchive copies a task and its related data from src to dst database.
func copyTaskToArchive(ctx context.Context, src, dst *sql.DB, taskID string) error {
	// Copy task using raw column values (avoids type parsing).
	copyRows(ctx, src, dst, "tasks",
		"SELECT "+taskColumns+" FROM tasks WHERE id = ?",
		`INSERT OR REPLACE INTO tasks (id, parent_id, title, description, status,
		 priority, assignee, task_type, labels, notes, estimate, due_at,
		 created_at, updated_at, closed_at, close_reason, created_by, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, 18)

	// Copy comments.
	copyRows(ctx, src, dst, "comments",
		"SELECT id, task_id, author, content, created_at, updated_at FROM comments WHERE task_id = ?",
		"INSERT OR REPLACE INTO comments (id, task_id, author, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		taskID, 6)

	// Copy checkpoints.
	copyRows(ctx, src, dst, "checkpoints",
		"SELECT id, task_id, author, done, decisions, next, blockers, created_at FROM checkpoints WHERE task_id = ?",
		"INSERT OR REPLACE INTO checkpoints (id, task_id, author, done, decisions, next, blockers, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		taskID, 8)

	// Copy custom attributes.
	copyRows(ctx, src, dst, "custom_attributes",
		"SELECT task_id, key, value, created_at, updated_at FROM custom_attributes WHERE task_id = ?",
		"INSERT OR REPLACE INTO custom_attributes (task_id, key, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		taskID, 5)

	return nil
}

// copyRows copies rows from src to dst for a given task.
func copyRows(ctx context.Context, src, dst *sql.DB, table, selectSQL, insertSQL, taskID string, numCols int) {
	rows, err := src.QueryContext(ctx, selectSQL, taskID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		vals := make([]any, numCols)
		ptrs := make([]any, numCols)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		dst.ExecContext(ctx, insertSQL, vals...)
	}
}
