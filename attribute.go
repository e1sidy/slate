package slate

import (
	"context"
	"encoding/json"
	"fmt"
)

// DefineAttr registers a custom attribute key with a type. Idempotent.
func (s *Store) DefineAttr(ctx context.Context, key string, attrType AttrType, desc string) error {
	if !attrType.IsValid() {
		return fmt.Errorf("invalid attribute type: %s", attrType)
	}

	// Check if already defined.
	var exists int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM attribute_definitions WHERE key = ?", key).Scan(&exists)
	if exists > 0 {
		return nil // idempotent
	}

	now := timeNowUTC()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO attribute_definitions (key, attr_type, description, created_at) VALUES (?, ?, ?, ?)",
		key, string(attrType), desc, now.Format(timeFormat),
	)
	if err != nil {
		return fmt.Errorf("define attr: %w", err)
	}
	return nil
}

// UndefineAttr removes an attribute definition and all its values.
func (s *Store) UndefineAttr(ctx context.Context, key string) error {
	s.db.ExecContext(ctx, "DELETE FROM custom_attributes WHERE key = ?", key)
	_, err := s.db.ExecContext(ctx, "DELETE FROM attribute_definitions WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("undefine attr: %w", err)
	}
	return nil
}

// GetAttrDef retrieves a single attribute definition.
func (s *Store) GetAttrDef(ctx context.Context, key string) (*AttrDefinition, error) {
	var ad AttrDefinition
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		"SELECT key, attr_type, description, created_at FROM attribute_definitions WHERE key = ?", key,
	).Scan(&ad.Key, &ad.Type, &ad.Description, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("attr def not found: %w", err)
	}
	if ct := strToTime(createdAt); ct != nil {
		ad.CreatedAt = *ct
	}
	return &ad, nil
}

// ListAttrDefs returns all attribute definitions.
func (s *Store) ListAttrDefs(ctx context.Context) ([]*AttrDefinition, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT key, attr_type, description, created_at FROM attribute_definitions ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("list attr defs: %w", err)
	}
	defer rows.Close()

	var defs []*AttrDefinition
	for rows.Next() {
		var ad AttrDefinition
		var createdAt string
		if err := rows.Scan(&ad.Key, &ad.Type, &ad.Description, &createdAt); err != nil {
			return nil, fmt.Errorf("scan attr def: %w", err)
		}
		if ct := strToTime(createdAt); ct != nil {
			ad.CreatedAt = *ct
		}
		defs = append(defs, &ad)
	}
	return defs, rows.Err()
}

// SetAttr sets a custom attribute on a task. The key must be defined first.
func (s *Store) SetAttr(ctx context.Context, taskID, key, value string) error {
	if err := s.validateTaskExists(taskID); err != nil {
		return err
	}

	// Validate against definition.
	def, err := s.GetAttrDef(ctx, key)
	if err != nil {
		return fmt.Errorf("attribute %q not defined: %w", key, err)
	}
	if err := validateAttrValue(def.Type, value); err != nil {
		return err
	}

	now := timeNowUTC()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO custom_attributes (task_id, key, value, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(task_id, key) DO UPDATE SET value = ?, updated_at = ?`,
		taskID, key, value, now.Format(timeFormat), now.Format(timeFormat),
		value, now.Format(timeFormat),
	)
	if err != nil {
		return fmt.Errorf("set attr: %w", err)
	}
	return nil
}

// GetAttr retrieves a single attribute for a task.
func (s *Store) GetAttr(ctx context.Context, taskID, key string) (*Attribute, error) {
	var a Attribute
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT ca.task_id, ca.key, ca.value, ad.attr_type, ca.created_at, ca.updated_at
		 FROM custom_attributes ca
		 JOIN attribute_definitions ad ON ca.key = ad.key
		 WHERE ca.task_id = ? AND ca.key = ?`, taskID, key,
	).Scan(&a.TaskID, &a.Key, &a.Value, &a.Type, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("get attr: %w", err)
	}
	if ct := strToTime(createdAt); ct != nil {
		a.CreatedAt = *ct
	}
	if ut := strToTime(updatedAt); ut != nil {
		a.UpdatedAt = *ut
	}
	return &a, nil
}

// DeleteAttr removes an attribute from a task.
func (s *Store) DeleteAttr(ctx context.Context, taskID, key string) error {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM custom_attributes WHERE task_id = ? AND key = ?", taskID, key)
	if err != nil {
		return fmt.Errorf("delete attr: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("attribute %s not found on task %s", key, taskID)
	}
	return nil
}

func validateAttrValue(attrType AttrType, value string) error {
	switch attrType {
	case AttrBoolean:
		if value != "true" && value != "false" {
			return fmt.Errorf("boolean attribute must be 'true' or 'false', got %q", value)
		}
	case AttrObject:
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("object attribute must be valid JSON, got %q", value)
		}
	}
	return nil
}

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
