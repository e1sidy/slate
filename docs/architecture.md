# Architecture

## Overview

Slate has two interfaces sharing one core:

```
┌─────────────┐     ┌─────────────┐
│   CLI       │     │  Go SDK     │
│  (cobra)    │     │  (import)   │
└──────┬──────┘     └──────┬──────┘
       └───────┬───────────┘
               │
        ┌──────▼──────┐
        │    Store     │  ← single entry point
        │  (store.go)  │
        └──────┬──────┘
               │
    ┌──────────┼──────────┬──────────┐
    │          │          │          │
┌───▼──┐  ┌───▼──┐  ┌───▼──────┐  ┌▼────────┐
│ task │  │ dep  │  │ comment  │  │attribute│
│ .go  │  │ .go  │  │ .go      │  │ .go     │
└───┬──┘  └───┬──┘  └────┬─────┘  └───┬─────┘
    └─────────┼──────────┼────────────┘
              │
       ┌──────▼──────┐
       │  event.go   │  ← every mutation writes here
       │  hook.go    │  ← then fires callbacks + shell hooks
       └──────┬──────┘
              │
       ┌──────▼──────┐
       │   SQLite    │
       │  (WAL mode) │
       └─────────────┘
```

## Data Flow

### Mutation Path

Every write follows this sequence:

```
SDK call (e.g., store.Create())
  → validate inputs
  → begin transaction (for Close/Cancel)
  → execute SQL (parameterized ? placeholders)
  → recordEvent()            ← writes to events table
    → emit()                 ← fires SDK callbacks (synchronous)
      → RunHooks()           ← fires shell hooks (async goroutine)
  → commit transaction
  → return result
```

### Query Path

```
SDK call (e.g., store.List())
  → build SQL WHERE clause from ListParams
  → execute parameterized query
  → scanTasks() / scanTaskRow()   ← string→time parsing
  → return []*Task
```

## Store Lifecycle

```go
store, err := slate.Open(ctx, dbPath, opts...)
defer store.Close()
```

`Open()` performs:
1. Create parent directories for the DB file
2. Open SQLite with `modernc.org/sqlite` (pure Go, no CGO)
3. `PRAGMA journal_mode=WAL` — concurrent reads
4. `PRAGMA busy_timeout=5000` — wait 5s when locked
5. `PRAGMA foreign_keys=ON` — referential integrity
6. Run versioned migrations from `internal/migrate/`
7. Apply options (prefix, hash length, config, lease timeout)

## Database Schema

### Tables

9 tables across 3 migrations:

#### Migration v1 — Core

| Table | Purpose |
|-------|---------|
| `tasks` | Primary entity. Title, status, priority, assignee, parent, labels, timestamps. Self-referencing FK via parent_id. |
| `comments` | Freeform text notes on tasks. Mutable (edit/delete). FK cascade to tasks. |
| `dependencies` | DAG edges between tasks. Composite PK (from_id, to_id). Types: blocks, relates_to, duplicates. FK cascade both directions. |
| `events` | Append-only audit log. Every mutation recorded with old/new values, actor, timestamp. AUTOINCREMENT id. |

#### Migration v2 — Custom Attributes

| Table | Purpose |
|-------|---------|
| `attribute_definitions` | Schema registry. Defines allowed attribute keys + types (string, boolean, object). |
| `custom_attributes` | Per-task typed key-value pairs. FK to both tasks (cascade) and definitions. Composite PK (task_id, key). |

#### Migration v3 — Checkpoints

| Table | Purpose |
|-------|---------|
| `checkpoints` | Structured progress snapshots (done, decisions, next, blockers). FK cascade to tasks. |
| `checkpoint_files` | File paths referenced in checkpoints. FK cascade to checkpoints. |

#### Infrastructure

| Table | Purpose |
|-------|---------|
| `schema_migrations` | Tracks which migration versions have been applied. |

### Column Reference

**tasks:**

| Column | Type | Default | Notes |
|--------|------|---------|-------|
| id | TEXT PK | — | Generated: `prefix-hash` or `parent.N` |
| parent_id | TEXT | NULL | FK to tasks(id), nullable |
| title | TEXT NOT NULL | — | Required |
| description | TEXT | '' | |
| status | TEXT NOT NULL | 'open' | open, in_progress, blocked, deferred, closed, cancelled |
| priority | INTEGER NOT NULL | 2 | 0=critical → 4=backlog |
| assignee | TEXT | '' | |
| task_type | TEXT NOT NULL | 'task' | task, bug, feature, epic, chore |
| labels | TEXT | '[]' | JSON array |
| notes | TEXT | '' | |
| estimate | INTEGER | 0 | Minutes |
| due_at | TEXT | '' | RFC3339 |
| created_at | TEXT NOT NULL | — | RFC3339 |
| updated_at | TEXT NOT NULL | — | RFC3339 |
| closed_at | TEXT | '' | RFC3339, NULL after reopen |
| close_reason | TEXT | '' | |
| created_by | TEXT | '' | |
| metadata | TEXT | '' | Arbitrary JSON |

**comments:** id, task_id, author, content, created_at, updated_at

**dependencies:** from_id, to_id, dep_type, created_at (PK: from_id + to_id)

**events:** id (AUTOINCREMENT), task_id, event_type, actor, field, old_value, new_value, timestamp

**attribute_definitions:** key (PK), attr_type, description, created_at

**custom_attributes:** task_id + key (PK), value, created_at, updated_at

**checkpoints:** id, task_id, author, done, decisions, next, blockers, created_at

**checkpoint_files:** checkpoint_id, file_path

### Indexes

```
idx_tasks_status          tasks(status)
idx_tasks_parent          tasks(parent_id)
idx_tasks_assignee        tasks(assignee)
idx_tasks_priority        tasks(priority)
idx_tasks_due             tasks(due_at)
idx_comments_task         comments(task_id)
idx_events_task           events(task_id)
idx_events_type           events(event_type)
idx_events_timestamp      events(timestamp)
idx_attrs_task            custom_attributes(task_id)
idx_attrs_key_value       custom_attributes(key, value)
idx_checkpoints_task      checkpoints(task_id)
idx_checkpoint_files      checkpoint_files(checkpoint_id)
```

## ID Generation

**Root tasks:**
```
prefix + "-" + sha256(uuid + timestamp)[:hashLen]
```
Example: `st-a3f8`, `st-c136`

**Subtasks (ladder notation):**
```
parentID + "." + childNumber
```
Example: `st-a3f8.1`, `st-a3f8.2`, `st-a3f8.1.1`

- Default prefix: `st` (configurable)
- Default hash length: 4 characters (configurable 3-8)
- Child numbering: sequential based on existing children count at creation time

## Migration System

- Each migration runs in its own database transaction
- Versions tracked in `schema_migrations` table
- Migrations are idempotent (skipped if already applied)
- Only additive — columns and tables are never removed

## File Layout

```
slate/
├── slate.go              # Core types, enums, params
├── store.go              # Store lifecycle, ID generation, event system
├── task.go               # Task mutations (Create, Close, Claim, etc.)
├── query.go              # Task queries (List, Ready, Blocked, etc.)
├── dependency.go         # DAG management, cycle detection
├── attribute.go          # Custom attribute CRUD + validation
├── comment.go            # Comment CRUD
├── checkpoint.go         # Checkpoint CRUD
├── event.go              # Event queries
├── hook.go               # Shell hook execution
├── config.go             # Config loading/saving
├── export.go             # JSONL export/import
├── metrics.go            # Metrics + Next
├── doctor.go             # Health checks
├── util.go               # Time/label helpers
├── internal/migrate/     # Schema migrations
├── cmd/slate/            # CLI (Cobra)
└── docs/                 # This documentation
```
