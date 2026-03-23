package slate

import (
	"context"
	"fmt"
)

// DiagnosticLevel indicates the severity of a diagnostic.
type DiagnosticLevel string

const (
	DiagOK   DiagnosticLevel = "ok"
	DiagWarn DiagnosticLevel = "warn"
	DiagFail DiagnosticLevel = "fail"
)

// Diagnostic is a single health check result.
type Diagnostic struct {
	Name    string
	Level   DiagnosticLevel
	Message string
}

// DoctorReport holds all health check results.
type DoctorReport struct {
	Diagnostics []Diagnostic
}

// HasIssues returns true if any diagnostic is not ok.
func (r *DoctorReport) HasIssues() bool {
	for _, d := range r.Diagnostics {
		if d.Level != DiagOK {
			return true
		}
	}
	return false
}

// Doctor runs health checks on the database and config.
func (s *Store) Doctor(ctx context.Context) (*DoctorReport, error) {
	report := &DoctorReport{}

	// 1. SQLite integrity check.
	var integrity string
	if err := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"integrity", DiagFail, fmt.Sprintf("integrity check failed: %v", err)})
	} else if integrity != "ok" {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"integrity", DiagFail, integrity})
	} else {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"integrity", DiagOK, "database integrity OK"})
	}

	// 2. Orphaned parent references.
	var orphanedParents int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks t
		 WHERE t.parent_id IS NOT NULL
		 AND NOT EXISTS (SELECT 1 FROM tasks p WHERE p.id = t.parent_id)`,
	).Scan(&orphanedParents)
	if orphanedParents > 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_parents", DiagWarn, fmt.Sprintf("%d tasks reference missing parents", orphanedParents)})
	} else {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_parents", DiagOK, "no orphaned parent references"})
	}

	// 3. Orphaned comments.
	var orphanedComments int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM comments c
		 WHERE NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = c.task_id)`,
	).Scan(&orphanedComments)
	if orphanedComments > 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_comments", DiagWarn, fmt.Sprintf("%d comments reference missing tasks", orphanedComments)})
	} else {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_comments", DiagOK, "no orphaned comments"})
	}

	// 4. Orphaned dependencies.
	var orphanedDeps int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dependencies d
		 WHERE NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.from_id)
		 OR NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.to_id)`,
	).Scan(&orphanedDeps)
	if orphanedDeps > 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_deps", DiagWarn, fmt.Sprintf("%d dependencies reference missing tasks", orphanedDeps)})
	} else {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"orphaned_deps", DiagOK, "no orphaned dependencies"})
	}

	// 5. Dependency cycles.
	cycles, err := s.DetectCycles(ctx)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"cycles", DiagWarn, fmt.Sprintf("cycle detection error: %v", err)})
	} else if len(cycles) > 0 {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"cycles", DiagWarn, fmt.Sprintf("%d dependency cycles detected", len(cycles))})
	} else {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{"cycles", DiagOK, "no dependency cycles"})
	}

	// 6. Config validation.
	if s.config != nil {
		if s.config.HashLen < 3 || s.config.HashLen > 8 {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{"config_hash_len", DiagWarn, fmt.Sprintf("hash_length %d is outside valid range 3-8", s.config.HashLen)})
		} else {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{"config_hash_len", DiagOK, "hash_length is valid"})
		}
	}

	// 7. Task summary.
	var total, open, inProgress, blocked, closed, cancelled int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&total)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'open'").Scan(&open)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'").Scan(&inProgress)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'blocked'").Scan(&blocked)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'closed'").Scan(&closed)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'cancelled'").Scan(&cancelled)

	report.Diagnostics = append(report.Diagnostics, Diagnostic{
		"task_summary", DiagOK,
		fmt.Sprintf("total=%d open=%d in_progress=%d blocked=%d closed=%d cancelled=%d",
			total, open, inProgress, blocked, closed, cancelled),
	})

	return report, nil
}
