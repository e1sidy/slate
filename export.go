package slate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// ExportJSONL writes all tasks, comments, dependencies, attributes, and checkpoints
// as newline-delimited JSON to the writer. Tasks are sorted by ID for deterministic output.
func (s *Store) ExportJSONL(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	// Export tasks.
	tasks, err := s.List(ctx, ListParams{})
	if err != nil {
		return fmt.Errorf("export tasks: %w", err)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })

	for _, t := range tasks {
		if err := writeJSONLine(bw, "task", t); err != nil {
			return err
		}
	}

	// Export comments.
	for _, t := range tasks {
		comments, err := s.ListComments(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, c := range comments {
			if err := writeJSONLine(bw, "comment", c); err != nil {
				return err
			}
		}
	}

	// Export dependencies.
	for _, t := range tasks {
		deps, err := s.ListDependencies(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, d := range deps {
			if err := writeJSONLine(bw, "dependency", d); err != nil {
				return err
			}
		}
	}

	// Export attribute definitions.
	rows, err := s.db.QueryContext(ctx,
		"SELECT key, attr_type, description, created_at FROM attribute_definitions ORDER BY key")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ad AttrDefinition
			var createdAt string
			if rows.Scan(&ad.Key, &ad.Type, &ad.Description, &createdAt) == nil {
				if ct := strToTime(createdAt); ct != nil {
					ad.CreatedAt = *ct
				}
				writeJSONLine(bw, "attr_definition", ad)
			}
		}
	}

	// Export custom attributes.
	for _, t := range tasks {
		attrs, err := s.Attrs(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, a := range attrs {
			if err := writeJSONLine(bw, "attribute", a); err != nil {
				return err
			}
		}
	}

	// Export checkpoints.
	for _, t := range tasks {
		cps, err := s.ListCheckpoints(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, cp := range cps {
			if err := writeJSONLine(bw, "checkpoint", cp); err != nil {
				return err
			}
		}
	}

	return nil
}

// ImportJSONL reads newline-delimited JSON and upserts all entities.
func (s *Store) ImportJSONL(ctx context.Context, r io.Reader) error {
	// Temporarily disable FK checks for import ordering.
	s.db.ExecContext(ctx, "PRAGMA foreign_keys=OFF")
	defer s.db.ExecContext(ctx, "PRAGMA foreign_keys=ON")

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB lines

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return fmt.Errorf("parse JSONL line: %w", err)
		}

		switch envelope.Type {
		case "task":
			var t Task
			if err := json.Unmarshal(envelope.Data, &t); err != nil {
				return fmt.Errorf("parse task: %w", err)
			}
			if err := s.upsertTask(ctx, &t); err != nil {
				return fmt.Errorf("upsert task %s: %w", t.ID, err)
			}
		case "comment":
			var c Comment
			if err := json.Unmarshal(envelope.Data, &c); err != nil {
				return fmt.Errorf("parse comment: %w", err)
			}
			s.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO comments (id, task_id, author, content, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				c.ID, c.TaskID, c.Author, c.Content,
				c.CreatedAt.Format(timeFormat), c.UpdatedAt.Format(timeFormat))
		case "dependency":
			var d Dependency
			if err := json.Unmarshal(envelope.Data, &d); err != nil {
				return fmt.Errorf("parse dependency: %w", err)
			}
			s.db.ExecContext(ctx,
				`INSERT OR IGNORE INTO dependencies (from_id, to_id, dep_type, created_at)
				 VALUES (?, ?, ?, ?)`,
				d.FromID, d.ToID, string(d.Type), d.CreatedAt.Format(timeFormat))
		case "attr_definition":
			var ad AttrDefinition
			if err := json.Unmarshal(envelope.Data, &ad); err != nil {
				return fmt.Errorf("parse attr def: %w", err)
			}
			s.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO attribute_definitions (key, attr_type, description, created_at)
				 VALUES (?, ?, ?, ?)`,
				ad.Key, string(ad.Type), ad.Description, ad.CreatedAt.Format(timeFormat))
		case "attribute":
			var a Attribute
			if err := json.Unmarshal(envelope.Data, &a); err != nil {
				return fmt.Errorf("parse attribute: %w", err)
			}
			s.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO custom_attributes (task_id, key, value, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?)`,
				a.TaskID, a.Key, a.Value, a.CreatedAt.Format(timeFormat), a.UpdatedAt.Format(timeFormat))
		case "checkpoint":
			var cp Checkpoint
			if err := json.Unmarshal(envelope.Data, &cp); err != nil {
				return fmt.Errorf("parse checkpoint: %w", err)
			}
			s.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO checkpoints (id, task_id, author, done, decisions, next, blockers, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				cp.ID, cp.TaskID, cp.Author, cp.Done, cp.Decisions, cp.Next, cp.Blockers,
				cp.CreatedAt.Format(timeFormat))
			for _, f := range cp.Files {
				s.db.ExecContext(ctx,
					"INSERT OR IGNORE INTO checkpoint_files (checkpoint_id, file_path) VALUES (?, ?)",
					cp.ID, f)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Rebuild FTS index after import.
	s.RebuildFTSIndex(ctx)

	return nil
}

// ExportEvents writes all events as JSONL.
func (s *Store) ExportEvents(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, event_type, actor, field, old_value, new_value, timestamp
		 FROM events ORDER BY timestamp ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("export events: %w", err)
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := writeJSONLine(bw, "event", e); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) upsertTask(ctx context.Context, t *Task) error {
	parentID := ""
	if t.ParentID != "" {
		parentID = t.ParentID
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO tasks (id, parent_id, title, description, status, priority,
		  assignee, task_type, labels, notes, estimate, due_at, created_at, updated_at,
		  closed_at, close_reason, created_by, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, nilIfEmpty(parentID), t.Title, t.Description, string(t.Status), int(t.Priority),
		t.Assignee, string(t.Type), labelsToJSON(t.Labels), t.Notes, t.Estimate,
		timeToStr(t.DueAt), t.CreatedAt.Format(timeFormat), t.UpdatedAt.Format(timeFormat),
		timeToStr(t.ClosedAt), t.CloseReason, t.CreatedBy, t.Metadata,
	)
	return err
}

func writeJSONLine(w *bufio.Writer, entityType string, data any) error {
	envelope := struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}{Type: entityType, Data: data}

	b, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", entityType, err)
	}
	w.Write(b)
	w.WriteByte('\n')
	return nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
