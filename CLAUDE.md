# Slate

Lightweight task management system — Go CLI + SDK backed by SQLite. Part of the Kite suite.

## Layout

```
slate/
├── slate.go              # Core types: Task, Comment, Checkpoint, Status, Priority, enums
├── store.go              # Store: Open/Close, ID generation, WAL+FK pragmas, event system
├── task.go               # Task mutations: Create, Get, Update, Close, Cancel, Reopen, Claim
├── query.go              # Task queries: List, Search, Ready, Blocked, Children, GetTree
├── dependency.go         # Dependency DAG: Add, Remove, cycle detection, tree visualization
├── attribute.go          # Custom attributes: Define, Set, Get, type validation
├── comment.go            # Comments: Add, Edit, Delete, List
├── checkpoint.go         # Checkpoints: Add, List, Latest
├── event.go              # Events, EventsSince (audit log)
├── hook.go               # Shell hook execution from config
├── config.go             # LoadConfig, SaveConfig, DefaultSlateHome, slate.yaml parsing
├── export.go             # ExportJSONL, ImportJSONL, ExportEvents (full export)
├── doctor.go             # Doctor(): health checks
├── util.go               # Time helpers, JSON label marshaling
├── internal/migrate/     # Versioned SQLite schema migrations (v1-v3)
├── cmd/slate/            # CLI (cobra) — thin wrapper over SDK
└── *_test.go             # SDK unit tests
```

## Build & Test

```bash
go build -o slate ./cmd/slate/    # Build binary
go test ./...                      # SDK unit tests
go vet ./...                       # Static analysis
```

## Conventions

- **SDK-first**: All logic in root package. CLI (`cmd/slate/`) is a thin cobra wrapper. Never put business logic in `cmd/`.
- **Module path**: `github.com/e1sidy/slate`
- All public SDK functions accept `context.Context` as first parameter.
- All public SDK functions return `(*Type, error)` or `error`.
- Use `UpdateParams` with pointer fields (nil = don't change) for partial updates.
- `SLATE_HOME` env var overrides `~/.slate/` as the central storage location.
- Error wrapping: `fmt.Errorf("context: %w", err)`. Never swallow errors.
- Parameterized SQL: all queries use `?` placeholders. Never `fmt.Sprintf` for SQL.
- Tests use `tempDB(t)` helper for isolated in-memory stores.
- Multi-step mutations (CloseTask, CancelTask) use `db.BeginTx` for atomicity.

## Status Model

```
open → in_progress → closed
  ↓         ↓          ↓
blocked   cancelled   (reopen → open)
  ↓
deferred
  ↓
  (auto-unblock → open when all blockers terminal)
```

- **`IsTerminal()`**: returns true for `closed` and `cancelled`. Always use this instead of `== StatusClosed`.
- **Auto-unblock**: closing/cancelling a task auto-transitions `blocked` dependents to `open` if all their blockers are terminal. Runs within the same transaction.
- **Auto-progress parent**: claiming a child auto-moves parent from `open` to `in_progress`.
- **Claim locking**: `Claim()` uses atomic `WHERE (assignee IS NULL OR assignee = '')` to prevent double-claim. Returns `ErrAlreadyClaimed` on conflict.
- **Lease expiry**: `ExpireLeases()` auto-releases stale claims after configurable timeout.
- **Icons**: `[ ]` open, `[>]` in_progress, `[!]` blocked, `[~]` deferred, `[x]` closed, `[-]` cancelled.

## What NOT to Do

- Don't add CGO dependencies — pure-Go SQLite is intentional.
- Don't put business logic in `cmd/slate/`.
- Don't scan `time.Time` directly from SQLite — scan as string, parse with `strToTime()`.
- Don't check `== StatusClosed` — use `IsTerminal()`.
- Don't break JSONL format without a major version bump.
- Don't remove columns from migrations — only add.
- Don't use `fmt.Sprintf` for SQL queries — always use `?` placeholders.
- Don't forget `context.Context` on new public methods.

## Key Types

- **Checkpoints** vs **Comments**: Checkpoints are structured snapshots (done, decisions, next, blockers, files). Comments are freeform and mutable (edit/delete supported).
- **Custom attributes**: Two-layer — definitions registry (key + type) → per-task values. Types: string, boolean, object.
- **Events**: Every mutation auto-records an event. DB write → event insert → SDK callbacks → shell hooks.
- **Transactions**: `CloseTask` and `CancelTask` wrap all DB operations in a transaction for crash safety.
- **Full export**: `ExportJSONL` exports tasks + comments + deps + attrs + checkpoints (not just tasks).
