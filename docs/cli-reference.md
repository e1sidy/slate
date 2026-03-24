# CLI Reference

## Global Flags

All commands accept these flags:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output as JSON |
| `--quiet` | bool | false | Minimal output (just IDs) |
| `--actor` | string | `cli` | Actor name for event attribution |
| `--timeout` | string | — | Operation timeout (e.g. `30s`, `5m`) |

---

## Task Management

### slate create

Create a new task.

```
slate create <title> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--desc` | string | — | Task description |
| `--type` | string | `task` | Task type: task, bug, feature, epic, chore |
| `--priority` | int | 2 | Priority: 0=critical, 1=high, 2=medium, 3=low, 4=backlog |
| `--assignee` | string | — | Assignee name |
| `--notes` | string | — | Freeform notes |
| `--labels` | string | — | Labels (comma-separated) |
| `--parent` | string | — | Parent task ID (creates subtask with ladder ID) |
| `--created-by` | string | — | Creator attribution |
| `--metadata` | string | — | Arbitrary JSON metadata |

**Examples:**
```bash
slate create "Fix login bug" --type bug --priority 1
slate create "Add OAuth" --type feature --assignee alice --labels "api,auth"
slate create "Implement endpoints" --parent st-a3f8
slate create "Quick task" --quiet  # prints only the ID
```

**Events:** `created`

---

### slate show

Show full task details including attributes, comments, dependencies, and latest checkpoint.

```
slate show <id>
```

**Examples:**
```bash
slate show st-a3f8
slate show st-a3f8 --json
```

---

### slate update

Update task fields. Use `--claim` for atomic claim operation.

```
slate update <id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--title` | string | — | New title |
| `--desc` | string | — | New description |
| `--status` | string | — | New status |
| `--priority` | int | — | New priority (0-4) |
| `--assignee` | string | — | New assignee |
| `--notes` | string | — | New notes |
| `--labels` | string | — | New labels (comma-separated) |
| `--parent` | string | — | Set parent task ID |
| `--orphan` | bool | false | Remove parent (mutually exclusive with --parent) |
| `--claim` | bool | false | Claim task (sets assignee + in_progress atomically) |

**Examples:**
```bash
slate update st-a3f8 --title "Updated title"
slate update st-a3f8 --priority 0 --assignee bob
slate update st-a3f8 --claim --actor agent-1
slate update st-a3f8 --status blocked
slate update st-a3f8 --orphan  # remove parent
```

**Events:** `updated` per field, `assigned`, `status_changed`
**Errors:** `ErrAlreadyClaimed` when using `--claim` and task is taken

---

### slate close

Close a task. Requires all children to be terminal.

```
slate close <id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--reason` | string | — | Close reason |

**Examples:**
```bash
slate close st-a3f8 --reason "Fixed in PR #42"
```

**Side effects:** Auto-unblocks dependents (within transaction).
**Errors:** Fails if any child is non-terminal.

---

### slate cancel

Cancel a task. Cascades to all non-terminal children.

```
slate cancel <id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--reason` | string | — | Cancel reason |

**Examples:**
```bash
slate cancel st-a3f8 --reason "Descoped from sprint"
```

**Side effects:** Cascades cancel to children. Auto-unblocks dependents.

---

### slate reopen

Reopen a closed or cancelled task.

```
slate reopen <id>
```

**Errors:** Fails if task is not terminal (closed or cancelled).

---

### slate delete

Permanently delete a task and all its children.

```
slate delete <id>
```

**Side effects:** Recursively deletes all descendants. Comments, deps, attrs removed via CASCADE. Events deleted from DB.

---

### slate search

Search tasks by title or description.

```
slate search <query>
```

**Examples:**
```bash
slate search "auth"
slate search "login" --json
```

---

## Querying

### slate list

List tasks with filters.

```
slate list [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--status` | string | — | Filter by status |
| `--assignee` | string | — | Filter by assignee |
| `--priority` | int | — | Filter by priority |
| `--type` | string | — | Filter by task type |
| `--label` | string | — | Filter by label (substring match) |
| `--parent` | string | — | Filter by parent ID |
| `--all` | bool | false | Include closed/cancelled tasks |
| `--tree` | bool | false | Hierarchical tree view |

**Default behavior:** Shows root tasks only (no parent), excludes closed/cancelled.

**Examples:**
```bash
slate list
slate list --tree
slate list --all
slate list --status in_progress --assignee alice
slate list --type bug --priority 1
slate list --json
```

---

### slate ready

List tasks with no unresolved blockers.

```
slate ready [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--parent` | string | — | Scope to children of this parent |

**Examples:**
```bash
slate ready
slate ready --parent st-a3f8
slate ready --json
```

---

### slate blocked

List tasks with status `blocked`.

```
slate blocked
```

---

### slate children

List direct children of a task.

```
slate children <id>
```

---

### slate next

Suggest the highest-impact ready task to work on.

```
slate next
```

Recommends the task that unblocks the most downstream work (by counting transitive dependents).

---

### slate events

Show event audit log for a task.

```
slate events <id>
```

**Examples:**
```bash
slate events st-a3f8
slate events st-a3f8 --json
```

---

## Dependencies

### slate dep add

Add a dependency (from depends on to).

```
slate dep add <from-id> <to-id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | `blocks` | Dependency type: blocks, relates_to, duplicates |

**Examples:**
```bash
slate dep add st-a3f8.2 st-a3f8.1              # .2 blocked by .1
slate dep add st-b1 st-a1 --type relates_to    # informational link
```

**Errors:** Rejects self-dependencies and cycles.

---

### slate dep remove

Remove a dependency.

```
slate dep remove <from-id> <to-id>
```

---

### slate dep list

List what a task depends on (its blockers).

```
slate dep list <id>
```

---

### slate dep tree

Show ASCII dependency tree.

```
slate dep tree <id>
```

Displays status icons: `[ ]` open, `[>]` in_progress, `[!]` blocked, `[~]` deferred, `[x]` closed, `[-]` cancelled.

---

### slate dep cycles

Detect dependency cycles in the graph.

```
slate dep cycles
```

---

## Custom Attributes

### slate attr define

Define a custom attribute key and type.

```
slate attr define <key> <type> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--desc` | string | — | Description of the attribute |

Types: `string`, `boolean`, `object`

**Examples:**
```bash
slate attr define env string --desc "Deployment environment"
slate attr define active boolean
slate attr define config object --desc "JSON configuration"
```

Idempotent — no error if already defined.

---

### slate attr undefine

Remove an attribute definition and all its values.

```
slate attr undefine <key>
```

---

### slate attr set

Set an attribute on a task.

```
slate attr set <task-id> <key> <value>
```

**Type validation:**
- `boolean`: value must be `"true"` or `"false"`
- `object`: value must be valid JSON

**Examples:**
```bash
slate attr set st-a3f8 env production
slate attr set st-a3f8 active true
slate attr set st-a3f8 config '{"retries": 3}'
```

---

### slate attr get

Get an attribute value from a task.

```
slate attr get <task-id> <key>
```

---

### slate attr delete

Remove an attribute from a task.

```
slate attr delete <task-id> <key>
```

---

### slate attr list

List all attribute definitions.

```
slate attr list
```

---

## Comments

### slate comment add

Add a comment to a task.

```
slate comment add <task-id> <content>
```

**Examples:**
```bash
slate comment add st-a3f8 "Found root cause in auth middleware"
```

---

### slate comment edit

Edit a comment.

```
slate comment edit <comment-id> <content>
```

---

### slate comment delete

Delete a comment.

```
slate comment delete <comment-id>
```

---

### slate comment list

List comments for a task.

```
slate comment list <task-id>
```

---

## Checkpoints

### slate checkpoint add

Add a structured progress checkpoint.

```
slate checkpoint add <task-id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--done` | string | — | What was accomplished (**required**) |
| `--decisions` | string | — | Key decisions and reasoning |
| `--next` | string | — | What should happen next |
| `--blockers` | string | — | Current blockers |
| `--files` | string[] | — | File paths touched (repeatable) |

**Examples:**
```bash
slate checkpoint add st-a3f8 \
  --done "Implemented JWT auth flow" \
  --decisions "Used RS256 over HS256 for key rotation" \
  --next "Add refresh token endpoint" \
  --files auth.go --files auth_test.go
```

---

### slate checkpoint list

List checkpoints for a task.

```
slate checkpoint list <task-id>
```

---

## Export / Import

### slate export

Export all data as JSONL.

```
slate export [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | stdout | Output file path |

Exports: tasks, comments, dependencies, attribute definitions, attributes, checkpoints.

**Examples:**
```bash
slate export --file backup.jsonl
slate export > backup.jsonl
```

---

### slate import

Import data from JSONL.

```
slate import <file>
```

**Examples:**
```bash
slate import backup.jsonl
```

---

## Metrics

### slate metrics

Show task metrics.

```
slate metrics [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from` | string | — | Start date (YYYY-MM-DD) |
| `--to` | string | — | End date (YYYY-MM-DD, inclusive) |
| `--actor` | string | — | Filter by actor |

**Examples:**
```bash
slate metrics
slate metrics --from 2026-03-01 --to 2026-03-23
slate metrics --actor agent-1
slate metrics --json
```

**Output fields:** TasksCreated, TasksClosed, TasksCancelled, CurrentOpen, CurrentBlocked, AvgCycleTime.

---

## Configuration

### slate config show

Show current configuration.

```
slate config show
```

---

### slate config set

Set a configuration value.

```
slate config set <key> <value>
```

| Key | Valid Values |
|-----|-------------|
| `prefix` | Any string |
| `hash_length` | 3-8 |
| `default_view` | `list`, `tree` |
| `show_all` | `true`, `false` |

---

### slate config get

Get a configuration value.

```
slate config get <key>
```

Keys: `prefix`, `hash_length`, `default_view`, `show_all`, `db_path`, `home`.

---

### slate config init

Initialize Slate home directory and config.

```
slate config init [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--prefix` | string | `st` | ID prefix |

Creates `~/.slate/`, writes default `slate.yaml`, initializes database. Safe to re-run.

---

## Utilities

### slate stats

Show task statistics by status.

```
slate stats
```

---

### slate doctor

Run health checks on database and config.

```
slate doctor
```

**Checks:** SQLite integrity, orphaned parent references, orphaned comments, orphaned dependencies, dependency cycles, config validation, task summary.

---

### slate version

Print version.

```
slate version
```

---

### slate completion

Generate shell completion scripts.

```
slate completion <shell>
```

Shells: `bash`, `zsh`, `fish`

**Setup:**
```bash
source <(slate completion bash)     # bash
source <(slate completion zsh)      # zsh
slate completion fish | source      # fish
```

Provides dynamic completion for task IDs, statuses, types, and priorities.

---

## Notion Sync

### slate notion connect

Connect to a Notion database.

```
slate notion connect [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--token` | string | — | Notion API token (**required**) |
| `--database-id` | string | — | Notion database ID (**required**) |
| `--auto` | bool | false | Auto-detect property mapping from database schema |

Validates the token and database access, caches workspace users, saves config to `~/.slate/notion.yaml`.

---

### slate notion disconnect

Remove Notion connection config.

```
slate notion disconnect
```

---

### slate notion status

Show connection status, property mapping, and sync stats.

```
slate notion status
```

---

### slate notion sync push

Push Slate tasks to Notion.

```
slate notion sync push [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--task` | string | — | Push a single task by ID |
| `--filter` | string | — | Filter tasks (e.g. `type:epic`, `status:open`) |

Creates new Notion pages or updates existing ones. Two-pass strategy for parent relations.

---

### slate notion sync pull

Pull Notion changes to Slate.

```
slate notion sync pull
```

Detects pages modified since last sync, updates local tasks, creates new tasks from unsynced pages, syncs comments.

---

### slate notion sync all

Full bidirectional sync.

```
slate notion sync all [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--filter` | string | — | Filter tasks for push |

Pushes local changes, pulls remote changes, detects and auto-resolves conflicts with last-write-wins.

---

### slate notion conflicts

List unresolved sync conflicts.

```
slate notion conflicts
```

---

### slate notion resolve

Manually resolve a sync conflict.

```
slate notion resolve <task-id> [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--prefer` | string | — | Resolution: `local` or `notion` (**required**) |

---

### slate notion dashboard

Push metrics dashboard to Notion.

```
slate notion dashboard [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--weekly` | bool | false | Create weekly digest instead of dashboard |
