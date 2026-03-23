package slate

import (
	"context"
	"fmt"
)

// Attrs returns all custom attributes for a task.
func (s *Store) Attrs(ctx context.Context, id string) ([]*Attribute, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ca.task_id, ca.key, ca.value, ad.attr_type, ca.created_at, ca.updated_at
		 FROM custom_attributes ca
		 JOIN attribute_definitions ad ON ca.key = ad.key
		 WHERE ca.task_id = ?
		 ORDER BY ca.key`, id)
	if err != nil {
		return nil, fmt.Errorf("list attrs: %w", err)
	}
	defer rows.Close()

	var attrs []*Attribute
	for rows.Next() {
		var a Attribute
		var createdAt, updatedAt string
		if err := rows.Scan(&a.TaskID, &a.Key, &a.Value, &a.Type, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan attr: %w", err)
		}
		if ct := strToTime(createdAt); ct != nil {
			a.CreatedAt = *ct
		}
		if ut := strToTime(updatedAt); ut != nil {
			a.UpdatedAt = *ut
		}
		attrs = append(attrs, &a)
	}
	return attrs, rows.Err()
}
