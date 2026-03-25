# Notion Sync Guide

Bidirectional sync between Slate and Notion. PMs see task status in Notion, can reprioritize and comment, and changes flow back to Slate.

## Setup

### 1. Get a Notion API Token

1. Go to [Notion Integrations](https://www.notion.so/my-integrations)
2. Create a new integration with read/write access
3. Copy the Internal Integration Token (`ntn_...`)

### 2. Share Your Database

1. Open your Notion database
2. Click "..." → "Connections" → Add your integration

### 3. Connect Slate

```bash
# Auto-detect property mapping from existing database schema (recommended):
slate notion connect --token ntn_... --database-id <id> --auto

# Or use default mapping (for fresh databases):
slate notion connect --token ntn_... --database-id <id>
```

The `--auto` flag reads your database schema and infers the property mapping, status mapping, and priority mapping.

### 4. Check Status

```bash
slate notion status
```

Shows connection info, property mapping, status mapping, and sync stats.

## Configuration

Connection config is stored in `~/.slate/notion.yaml` (separate from `slate.yaml` for security, with `0600` permissions).

### Property Mapping

Maps Slate fields to Notion property names. Set to empty string to skip or auto-create:

```yaml
property_map:
  title: "Task name"       # title type
  status: "Status"         # status type
  priority: "Priority"     # select type
  assignee: "Assignee"     # people type
  labels: "Tags"           # multi_select type
  due_at: "Due Date"       # date type
  parent_id: "Parent-task" # relation type (self)
  description: ""          # empty = use page body
  type: ""                 # empty = auto-create
  progress: ""             # empty = auto-create
```

### Dependency Mapping

Maps Slate dependency types to Notion relation properties:

```yaml
dep_map:
  blocks: "Blocked by"
  relates_to: "Related to"
```

### Status Mapping

Maps between Slate statuses and Notion status options. Multiple Notion values can map to one Slate status (many-to-one). The first value is used for push:

```yaml
status_map:
  open: ["Todo"]
  in_progress: ["In Progress", "In Review", "On QA", "Verify"]
  blocked: ["Blocked", "RCA Pending"]
  closed: ["Done", "Verified on QA"]
  cancelled: ["Cancelled"]
  deferred: []  # no Notion equivalent
```

### Priority Mapping

```yaml
priority_map:
  0: "High"     # P0 → High
  1: "High"     # P1 → High
  2: "Medium"   # P2 → Medium
  3: "Low"      # P3 → Low
  4: "Low"      # P4 → Low

priority_reverse:
  "High": 1     # High → P1
  "Medium": 2   # Medium → P2
  "Low": 3      # Low → P3
```

### Auto-Create Properties

When `auto_create_properties: true` (default), missing Notion properties are created on first sync. Properties that already exist are used as-is. Type mismatches produce warnings.

## Sync Commands

### Push (Slate → Notion)

```bash
# Push all tasks
slate notion sync push

# Push a single task
slate notion sync push --task st-ab12

# Push only epics
slate notion sync push --filter "type:epic"
```

Creates Notion pages for new tasks, updates existing pages. Uses two-pass strategy: parents first, then children (for parent relations).

### Pull (Notion → Slate)

```bash
# Pull all tasks (filtered by user_id if set)
slate notion sync pull

# Pull only current sprint's tasks (auto-detects sprint)
slate notion sync pull --sprint=current

# Pull a specific sprint by page ID
slate notion sync pull --sprint=2c4b8ace-64ab-8020-8337-dde3acac1019
```

Detects Notion pages modified since last sync. Updates local tasks, creates new tasks from unsynced pages, syncs comments.

**Bidirectional fields** (pulled from Notion): status, priority, assignee, labels, due_at, parent_id, description.

**Read-only fields** (never pulled): id, type.

### Bidirectional Sync

```bash
# Full sync: push + pull + conflict detection
slate notion sync all

# With filter
slate notion sync all --filter "type:feature"
```

### Selective Sync

Filter syntax: `key:value` pairs separated by spaces.

```bash
slate notion sync push --filter "type:epic"
slate notion sync push --filter "status:open"
slate notion sync push --filter "assignee:alice"
slate notion sync push --filter "type:bug status:open"
```

## Conflict Resolution

When both Slate and Notion change the same field between syncs, a conflict is detected.

### Default: Last-Write-Wins

The most recently modified version wins automatically. Conflicts are logged in the sync table for auditing.

### View Conflicts

```bash
slate notion conflicts
```

### Manual Resolution

```bash
# Overwrite Notion with Slate values
slate notion resolve st-ab12 --prefer local

# Overwrite Slate with Notion values
slate notion resolve st-ab12 --prefer notion
```

## Parent Relations

Parent-child relationships are **bidirectional**. Parents assigned in Notion flow back to Slate on pull.

- **Push**: Parents pushed before children (two-pass). Parent relation set via Notion self-relation property.
- **Pull**: Parent relation resolved by looking up the parent's Notion page ID in the sync table. If parent isn't synced yet, it's pulled first (recursive).
- **Unparent**: Clearing the parent relation in Notion sets `parent_id = ""` in Slate on pull.

## Dependencies

Slate `blocks` and `relates_to` dependencies sync to Notion relation properties:

- `blocks` → "Blocked by" / "Is blocking"
- `relates_to` → "Related to" / "Relates to"

Dependencies are synced bidirectionally: push creates Notion relations, pull creates/removes Slate dependencies.

## Dashboard

```bash
# Create/update metrics dashboard page
slate notion dashboard

# Create weekly digest (new page each week)
slate notion dashboard --weekly
```

**Dashboard contents**: open/blocked counts, tasks closed/created, average cycle time.

**Weekly digest**: completed tasks, created tasks, key decisions from checkpoints.

## Sprint Filtering

Filter synced tasks to a specific sprint. Requires a Notion Sprint relation property linking to a sprints database.

### Configuration

```yaml
# In ~/.slate/notion.yaml:
sprint_property: "Sprint"                              # Notion relation property name (default: "Sprint")
sprint_database_id: "855124ff-3de5-4713-ae40-..."      # Auto-detected from the relation if not set
sprint_id: "auto"                                      # "auto" = detect current sprint each pull
```

### Auto-Detection

When `sprint_id: auto` or `--sprint=current` is used, Slate:

1. Reads the task database schema to find the Sprint relation property
2. Follows the relation to the sprints database
3. Queries for a sprint with status "Current" (falls back to "In Progress")
4. Filters the pull query to only tasks linked to that sprint

The sprint database ID is cached in `notion.yaml` after first detection to avoid repeated lookups.

### CLI Usage

```bash
# Auto-detect current sprint for this pull
slate notion sync pull --sprint=current

# Use a specific sprint page ID
slate notion sync pull --sprint=<sprint-page-id>
```

The `--sprint` flag overrides the `sprint_id` config for that run. Combined with `user_id`, the pull filters by **both** assignee AND sprint (compound AND filter).

### Persistent Configuration

To always filter by the current sprint, set in `notion.yaml`:

```yaml
sprint_id: auto
```

Every pull will auto-detect the current sprint before querying.

## Rate Limiting

Notion API limit is 3 requests/second. Configure in `notion.yaml`:

```yaml
rate_limit: 334ms  # ~3 req/sec (default)
```

## Disconnect

```bash
slate notion disconnect
```

Removes `notion.yaml`. Does not delete Notion pages or sync records from the database.
