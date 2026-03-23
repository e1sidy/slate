# Export Format

## Overview

Slate uses newline-delimited JSON (JSONL) for data export and import. Each line is a self-contained JSON object with a type envelope.

## Envelope Format

```json
{"type": "<entity_type>", "data": { ... }}
```

## Entity Types

### task

```json
{
  "type": "task",
  "data": {
    "id": "st-a3f8",
    "parent_id": "",
    "title": "Fix login bug",
    "description": "JWT token not refreshing",
    "status": "open",
    "priority": 1,
    "assignee": "alice",
    "type": "bug",
    "labels": ["api", "auth"],
    "notes": "",
    "estimate": 120,
    "due_at": "2026-04-01T00:00:00Z",
    "created_at": "2026-03-23T14:30:00Z",
    "updated_at": "2026-03-23T15:00:00Z",
    "closed_at": null,
    "close_reason": "",
    "created_by": "agent-1",
    "metadata": "{\"source\": \"jira\"}"
  }
}
```

### comment

```json
{
  "type": "comment",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "task_id": "st-a3f8",
    "author": "alice",
    "content": "Found the root cause",
    "created_at": "2026-03-23T15:00:00Z",
    "updated_at": "2026-03-23T15:00:00Z"
  }
}
```

### dependency

```json
{
  "type": "dependency",
  "data": {
    "from_id": "st-a3f8.2",
    "to_id": "st-a3f8.1",
    "type": "blocks",
    "created_at": "2026-03-23T14:35:00Z"
  }
}
```

### attr_definition

```json
{
  "type": "attr_definition",
  "data": {
    "key": "env",
    "type": "string",
    "description": "Deployment environment",
    "created_at": "2026-03-23T14:30:00Z"
  }
}
```

### attribute

```json
{
  "type": "attribute",
  "data": {
    "task_id": "st-a3f8",
    "key": "env",
    "value": "production",
    "type": "string",
    "created_at": "2026-03-23T14:30:00Z",
    "updated_at": "2026-03-23T14:30:00Z"
  }
}
```

### checkpoint

```json
{
  "type": "checkpoint",
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440000",
    "task_id": "st-a3f8",
    "author": "agent-1",
    "done": "Implemented auth flow",
    "decisions": "Used JWT over sessions",
    "next": "Add integration tests",
    "blockers": "",
    "files": ["auth.go", "auth_test.go"],
    "created_at": "2026-03-23T16:00:00Z"
  }
}
```

### event (export only)

```json
{
  "type": "event",
  "data": {
    "id": 1,
    "task_id": "st-a3f8",
    "event_type": "status_changed",
    "actor": "agent-1",
    "field": "status",
    "old_value": "open",
    "new_value": "in_progress",
    "timestamp": "2026-03-23T15:30:00Z"
  }
}
```

Events are exported via `slate export-events` (separate from main export) and are not imported.

## Export Order

`ExportJSONL` writes entities in this order:

1. All tasks (sorted by ID for deterministic git diffs)
2. All comments (grouped by task)
3. All dependencies (grouped by from_id)
4. All attribute definitions (sorted by key)
5. All custom attributes (grouped by task)
6. All checkpoints (grouped by task, with files embedded)

## Import Behavior

`ImportJSONL` processes a JSONL file line by line:

- **FK checks disabled** during import (`PRAGMA foreign_keys=OFF`) to handle arbitrary line ordering
- **FK checks re-enabled** after import
- **Tasks:** `INSERT OR REPLACE` (upsert — updates existing, inserts new)
- **Comments:** `INSERT OR REPLACE`
- **Dependencies:** `INSERT OR IGNORE` (skip duplicates)
- **Attribute definitions:** `INSERT OR REPLACE`
- **Custom attributes:** `INSERT OR REPLACE`
- **Checkpoints:** `INSERT OR REPLACE`
- **Checkpoint files:** `INSERT OR IGNORE`
- **Buffer:** 1MB per line (handles large checkpoints)

## Round-Trip Guarantee

Export followed by import into an empty database produces an equivalent state for tasks, comments, dependencies, attributes, and checkpoints. Events are NOT included in the main export (use `ExportEvents` separately).

## CLI Usage

```bash
# Export to file
slate export --file backup.jsonl

# Export to stdout
slate export

# Import from file
slate import backup.jsonl

# Export events separately
# (via SDK only — no CLI command for event export currently)
```
