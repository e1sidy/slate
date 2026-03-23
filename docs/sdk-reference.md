# SDK Reference

## Installation

```bash
go get github.com/e1sidy/slate
```

## Store Lifecycle

### Open

```go
func Open(ctx context.Context, dbPath string, opts ...Option) (*Store, error)
```

Creates or opens a Slate database. Runs migrations, enables WAL mode and foreign keys.

```go
store, err := slate.Open(ctx, "tasks.db", slate.WithPrefix("st"))
if err != nil { log.Fatal(err) }
defer store.Close()
```

### Options

| Option | Description |
|--------|-------------|
| `WithPrefix(p string)` | ID prefix (default: "st") |
| `WithHashLength(n int)` | Hash length 3-8 (default: 4) |
| `WithConfig(c *Config)` | Attach parsed config (sets prefix, hash, lease from config) |
| `WithLeaseTimeout(d time.Duration)` | Claim expiry duration (default: 30m) |

### Close

```go
func (s *Store) Close() error
```

### Helpers

```go
func (s *Store) DB() *sql.DB       // underlying database
func (s *Store) Prefix() string    // configured prefix
func GenerateID(prefix string, hashLen int) string  // generate ID without a store
```

---

## Task Operations

### Create

```go
func (s *Store) Create(ctx context.Context, p CreateParams) (*Task, error)
```

Creates a new task. If `ParentID` is set, generates a ladder ID (`parent.N`).

**CreateParams fields:** Title (required), Description, Type, Priority, ParentID, Assignee, Labels, Notes, Estimate, DueAt, CreatedBy, Metadata.

**Events:** `EventCreated`

### Get / GetFull

```go
func (s *Store) Get(ctx context.Context, id string) (*Task, error)
func (s *Store) GetFull(ctx context.Context, id string) (*Task, error)
```

`Get` retrieves a task by ID. `GetFull` also populates the `Attrs` map with custom attributes.

### Update

```go
func (s *Store) Update(ctx context.Context, id string, p UpdateParams, actor string) (*Task, error)
```

Partial update — nil pointer fields are skipped. Records an event for each changed field.

**UpdateParams fields:** Title, Description, Priority, Assignee, Labels, Notes, Estimate, DueAt, Metadata (all `*type`), ParentID (`*string`), Orphan (`bool`).

**Events:** `EventUpdated` per field, `EventAssigned` for assignee changes

### UpdateStatus

```go
func (s *Store) UpdateStatus(ctx context.Context, id string, status Status, actor string) error
```

Changes status. Auto-progresses parent if transitioning to `in_progress`.

**Events:** `EventStatusChanged`

### CloseTask

```go
func (s *Store) CloseTask(ctx context.Context, id string, reason string, actor string) error
```

Closes a task within a transaction. Rejects if any child is non-terminal. Auto-unblocks dependents.

**Events:** `EventClosed`
**Errors:** Returns error if children are not all terminal.

### CancelTask

```go
func (s *Store) CancelTask(ctx context.Context, id string, reason string, actor string) error
```

Cancels a task within a transaction. Cascades to all non-terminal children. Auto-unblocks dependents.

**Events:** `EventStatusChanged` for each cancelled task

### Reopen

```go
func (s *Store) Reopen(ctx context.Context, id string, actor string) error
```

Reopens a terminal task. Sets status to `open`, clears `closed_at` and `close_reason`.

**Events:** `EventStatusChanged`
**Errors:** Returns error if task is not terminal.

### DeleteTask

```go
func (s *Store) DeleteTask(ctx context.Context, id string, actor string) error
```

Permanently deletes a task and all descendants (depth-first). Events are deleted from DB. A `EventDeleted` event is emitted to listeners only (not stored).

### Claim

```go
func (s *Store) Claim(ctx context.Context, id string, assignee string) (*ClaimResult, error)
```

Atomically sets assignee and status to `in_progress`. Returns `ErrAlreadyClaimed` if another agent holds the claim.

**ClaimResult:** `ParentProgressed bool`, `ParentID string`

### ReleaseClaim

```go
func (s *Store) ReleaseClaim(ctx context.Context, id string, actor string) error
```

Releases a claim — sets assignee to empty, status to `open`.

### ExpireLeases

```go
func (s *Store) ExpireLeases(ctx context.Context) (int, error)
```

Auto-releases stale claims where `updated_at` is older than lease timeout. Returns count of released tasks.

### CloseMany / UpdateMany

```go
func (s *Store) CloseMany(ctx context.Context, ids []string, reason string, actor string) (int, error)
func (s *Store) UpdateMany(ctx context.Context, filter ListParams, p UpdateParams, actor string) (int, error)
```

Bulk operations. `CloseMany` closes tasks by ID list. `UpdateMany` updates all tasks matching a filter.

---

## Query Operations

### List

```go
func (s *Store) List(ctx context.Context, p ListParams) ([]*Task, error)
```

Returns tasks matching filters, ordered by priority ASC, created_at DESC.

**ListParams fields:** Status, Assignee, Priority, ParentID (`*string` — nil=all, ""=root only), Type, Label, AttrFilter (`map[string]string`), ExcludeStatuses, Limit, Offset.

### Search

```go
func (s *Store) Search(ctx context.Context, query string) ([]*Task, error)
```

LIKE search on title and description.

### Ready

```go
func (s *Store) Ready(ctx context.Context, parentID string) ([]*Task, error)
```

Open tasks with no unresolved `blocks` dependencies. If `parentID` is non-empty, scoped to that parent's children.

### Blocked

```go
func (s *Store) Blocked(ctx context.Context) ([]*Task, error)
```

Tasks with status `blocked`.

### Children

```go
func (s *Store) Children(ctx context.Context, id string) ([]*Task, error)
```

Direct children of a task.

### GetTree

```go
func (s *Store) GetTree(ctx context.Context, id string) (*Task, error)
```

Recursively populates the `Children` field for the entire subtree.

### Next

```go
func (s *Store) Next(ctx context.Context) (*Task, error)
```

Suggests the highest-impact ready task by counting transitive dependents via BFS.

---

## Dependencies

### AddDependency

```go
func (s *Store) AddDependency(ctx context.Context, fromID, toID string, depType DepType) error
```

Creates a dependency edge. Validates: no self-deps, no duplicates, no cycles (BFS). Idempotent for duplicates.

**Events:** `EventDependencyAdded`

### RemoveDependency

```go
func (s *Store) RemoveDependency(ctx context.Context, fromID, toID string) error
```

**Events:** `EventDependencyRemoved` (if row existed)

### ListDependencies / ListDependents

```go
func (s *Store) ListDependencies(ctx context.Context, id string) ([]*Dependency, error)  // what I depend on
func (s *Store) ListDependents(ctx context.Context, id string) ([]*Dependency, error)    // what depends on me
```

### DepTree

```go
func (s *Store) DepTree(ctx context.Context, id string) (string, error)
```

Returns ASCII tree visualization with status icons.

### DetectCycles

```go
func (s *Store) DetectCycles(ctx context.Context) ([][]string, error)
```

Full-graph DFS cycle detection. Returns list of cycles (each a slice of task IDs).

---

## Custom Attributes

### DefineAttr / UndefineAttr

```go
func (s *Store) DefineAttr(ctx context.Context, key string, attrType AttrType, desc string) error
func (s *Store) UndefineAttr(ctx context.Context, key string) error
```

`DefineAttr` is idempotent. `UndefineAttr` deletes all per-task values first.

### SetAttr / GetAttr / DeleteAttr

```go
func (s *Store) SetAttr(ctx context.Context, taskID, key, value string) error
func (s *Store) GetAttr(ctx context.Context, taskID, key string) (*Attribute, error)
func (s *Store) DeleteAttr(ctx context.Context, taskID, key string) error
```

`SetAttr` validates type (boolean: "true"/"false", object: valid JSON). Uses upsert.

### GetAttrDef / ListAttrDefs / Attrs

```go
func (s *Store) GetAttrDef(ctx context.Context, key string) (*AttrDefinition, error)
func (s *Store) ListAttrDefs(ctx context.Context) ([]*AttrDefinition, error)
func (s *Store) Attrs(ctx context.Context, id string) ([]*Attribute, error)
```

### Attribute Methods

```go
func (a *Attribute) BoolValue() bool
func (a *Attribute) StringValue() string
func (a *Attribute) ObjectValue() (map[string]any, error)
```

---

## Comments

```go
func (s *Store) AddComment(ctx context.Context, taskID, author, content string) (*Comment, error)
func (s *Store) EditComment(ctx context.Context, commentID, content string) error
func (s *Store) DeleteComment(ctx context.Context, commentID string) error
func (s *Store) ListComments(ctx context.Context, taskID string) ([]*Comment, error)
```

Comments are mutable. `AddComment` records `EventCommented`.

---

## Checkpoints

```go
func (s *Store) AddCheckpoint(ctx context.Context, taskID, author string, p CheckpointParams) (*Checkpoint, error)
func (s *Store) ListCheckpoints(ctx context.Context, taskID string) ([]*Checkpoint, error)
func (s *Store) LatestCheckpoint(ctx context.Context, taskID string) (*Checkpoint, error)
```

**CheckpointParams:** Done (required), Decisions, Next, Blockers, Files (`[]string`).

---

## Events

```go
func (s *Store) Events(ctx context.Context, taskID string) ([]*Event, error)
func (s *Store) EventsSince(ctx context.Context, taskID string, since time.Time) ([]*Event, error)
func (s *Store) On(eventType EventType, fn func(Event))
func (s *Store) Off(eventType EventType)
```

`On` registers a synchronous callback. `Off` removes all callbacks for a type.

---

## Export / Import

```go
func (s *Store) ExportJSONL(ctx context.Context, w io.Writer) error
func (s *Store) ImportJSONL(ctx context.Context, r io.Reader) error
func (s *Store) ExportEvents(ctx context.Context, w io.Writer) error
```

`ExportJSONL` exports all entities (tasks, comments, deps, attrs, checkpoints). `ImportJSONL` upserts with FK checks temporarily disabled. See [Export Format](export-format.md).

---

## Metrics

```go
func (s *Store) Metrics(ctx context.Context, p MetricsParams) (*MetricsReport, error)
func (s *Store) CycleTime(ctx context.Context, taskID string) (time.Duration, error)
func (s *Store) Throughput(ctx context.Context, from, to time.Time) (int, error)
func (s *Store) Next(ctx context.Context) (*Task, error)
```

**MetricsParams:** From, To (`*time.Time`), Actor (`string`).

**MetricsReport:** TasksCreated, TasksClosed, TasksCancelled, CurrentOpen, CurrentBlocked, AvgCycleTime.

---

## Diagnostics

```go
func (s *Store) Doctor(ctx context.Context) (*DoctorReport, error)
```

Runs 7 health checks: integrity, orphaned parents/comments/deps, cycles, config validation, task summary.

**DoctorReport:** `Diagnostics []Diagnostic`, `HasIssues() bool`.

---

## Hooks

```go
func RunHooks(cfg *Config, event Event)
func EnableHooks(store *Store, cfg *Config)
```

`EnableHooks` registers a catch-all listener that fires `RunHooks` for every event type. See [Configuration](configuration.md) for hook setup.

---

## Config

```go
func LoadConfig(path string) (*Config, error)
func SaveConfig(path string, cfg *Config) error
func DefaultSlateHome() string
func DefaultDBPath() string
func DefaultConfigPath() string
func DefaultConfig() Config
```

`LoadConfig("")` loads from default path. Returns `DefaultConfig()` if file doesn't exist.

---

## Types Quick Reference

| Type | Values |
|------|--------|
| `Status` | open, in_progress, blocked, deferred, closed, cancelled |
| `Priority` | P0 (0) through P4 (4) |
| `TaskType` | task, bug, feature, epic, chore |
| `DepType` | blocks, relates_to, duplicates |
| `EventType` | created, updated, status_changed, commented, assigned, closed, dependency_added, dependency_removed, deleted |
| `AttrType` | string, boolean, object |
| `DiagnosticLevel` | ok, warn, fail |
