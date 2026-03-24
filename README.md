# Slate

**Lightweight, embeddable task management system for Go — CLI + SDK backed by SQLite.**

Part of the [Kite](https://github.com/e1sidy) suite.

## Quick Start

```bash
# Install
go install github.com/e1sidy/slate/cmd/slate@latest

# Initialize
slate config init --prefix st

# Start tracking
slate create "Fix login bug" --type bug --priority 1 --assignee alice
slate create "Add OAuth" --type feature --priority 2
slate list
slate show st-a3f8
slate close st-a3f8 --reason "Fixed in commit abc123"
```

## Features

- **SDK-first** — the CLI is a thin wrapper; import `github.com/e1sidy/slate` in any Go app.
- **Pure Go SQLite** — single binary, no CGO, no runtime dependencies.
- **Task hierarchy** — parent/child tasks with ladder IDs (`st-a3f8.1`, `.1.1`).
- **Dependency DAG** — with automatic cycle detection and auto-unblock.
- **Atomic claim locking** — prevents double-claim in multi-agent scenarios.
- **Lease-based claims** — auto-release stale claims from crashed agents.
- **Transactional mutations** — close/cancel operations are crash-safe.
- **Custom attributes** — typed key-value metadata (string, boolean, JSON object).
- **Event system** — every mutation recorded. SDK callbacks, shell hooks, queryable audit log.
- **Mutable comments** — add, edit, delete (not just append-only).
- **Structured checkpoints** — progress snapshots with done/decisions/next/blockers/files.
- **Full export** — JSONL backup of tasks + comments + deps + attrs + checkpoints.
- **Notion bidirectional sync** — push/pull tasks, comments, parent relations, and dependencies to Notion.
- **Configurable field mapping** — map Slate fields to existing Notion properties with status/priority translation.
- **Conflict resolution** — field-level conflict detection with last-write-wins and manual override.
- **Metrics dashboard** — push task metrics and weekly digests to Notion pages.
- **Agent-friendly** — `--json` output on all commands, `--actor` flag for attribution.
- **Shell completions** — dynamic tab-completion for task IDs, flags, and attribute keys.
- **Health checks** — `slate doctor` validates database integrity.

## Essential Commands

| Command | Action |
|---------|--------|
| `slate create "Title" --priority 1` | Create a task |
| `slate list` | List open tasks |
| `slate list --tree` | Hierarchical tree view |
| `slate show <id>` | Task details, comments, deps |
| `slate update <id> --claim` | Atomically claim a task |
| `slate close <id> --reason "done"` | Close a task |
| `slate dep add <task> <blocker>` | Add a dependency |
| `slate ready` | Tasks with no unresolved blockers |
| `slate search <query>` | Search titles and descriptions |
| `slate doctor` | Run health checks |
| `slate notion connect --auto` | Connect to Notion database |
| `slate notion sync push` | Push tasks to Notion |
| `slate notion sync pull` | Pull changes from Notion |
| `slate notion dashboard` | Push metrics to Notion |

## SDK Usage

```go
import "github.com/e1sidy/slate"

store, _ := slate.Open(ctx, "tasks.db", slate.WithPrefix("st"))
defer store.Close()

task, _ := store.Create(ctx, slate.CreateParams{
    Title:    "Implement feature X",
    Type:     slate.TypeFeature,
    Priority: slate.P1,
})

// Atomic claim — returns ErrAlreadyClaimed if taken
result, err := store.Claim(ctx, task.ID, "agent-1")

// Events
store.On(slate.EventStatusChanged, func(e slate.Event) {
    fmt.Printf("Task %s: %s -> %s\n", e.TaskID, e.OldValue, e.NewValue)
})

store.CloseTask(ctx, task.ID, "done", "agent-1") // transactional
```

## Documentation

| Doc | Contents |
|-----|----------|
| [Getting Started](docs/getting-started.md) | Installation, first task, 5-minute walkthrough |
| [CLI Reference](docs/cli-reference.md) | Every command, flag, and example |
| [SDK Reference](docs/sdk-reference.md) | Full API for Go embedding |
| [Concepts](docs/concepts.md) | Status model, hierarchy, dependencies, claims, events |
| [Architecture](docs/architecture.md) | System design, data flow, schema |
| [Configuration](docs/configuration.md) | Config file, hooks, env vars |
| [Design Decisions](docs/design-decisions.md) | Why we chose what we chose |
| [Notion Sync](docs/notion-sync.md) | Setup, field mapping, sync, conflicts, dashboard |
| [Export Format](docs/export-format.md) | JSONL spec for backup/sync |

## Requirements

Go 1.22+

## License

MIT
