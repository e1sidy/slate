# Configuration

## File Location

Config is stored at `~/.slate/slate.yaml`. Override the home directory with the `SLATE_HOME` environment variable.

```bash
export SLATE_HOME=/path/to/custom/home
```

## Default Paths

| Path | Purpose |
|------|---------|
| `~/.slate/` | Home directory |
| `~/.slate/slate.yaml` | Config file |
| `~/.slate/slate.db` | SQLite database |
| `~/.slate/hooks.log` | Hook error log |

## Config Fields

| Field | Type | Default | Valid Values | Description |
|-------|------|---------|-------------|-------------|
| `prefix` | string | `st` | Any non-empty string | ID prefix for generated task IDs |
| `db_path` | string | `~/.slate/slate.db` | Any file path | SQLite database location |
| `hash_length` | int | `4` | 3–8 | Number of hex characters in generated IDs |
| `default_view` | string | `""` (list) | `list`, `tree` | Default display mode for `slate list` |
| `show_all` | bool | `false` | `true`, `false` | Include closed/cancelled tasks in list by default |
| `lease_timeout` | duration | `30m` | Go duration string | Auto-release claims after this period of inactivity |
| `hooks` | object | `{}` | See below | Shell hook configuration |

## Example Configs

### Minimal

```yaml
prefix: st
```

### Full

```yaml
prefix: myapp
db_path: /home/user/.slate/myapp.db
hash_length: 6
default_view: tree
show_all: false
lease_timeout: 15m
hooks:
  on_status_change:
    - command: "echo Task {id} changed from {old} to {new}"
      filter:
        new_status: "closed"
  on_create:
    - command: "slack-notify 'New task: {id}'"
  on_assign:
    - command: "email-notify {actor} assigned to {id}"
```

## Managing Config via CLI

```bash
# View current config
slate config show

# Set a value
slate config set prefix myapp
slate config set hash_length 6
slate config set default_view tree
slate config set show_all true

# Get a single value
slate config get prefix
slate config get home

# Initialize home directory and config
slate config init --prefix myapp
```

## Hook Configuration

### Event Types

| Config Key | Triggers On |
|------------|-------------|
| `on_status_change` | Any status transition |
| `on_create` | Task created |
| `on_comment` | Comment added |
| `on_close` | Task closed |
| `on_assign` | Assignee changed |

### Hook Definition

Each hook has a `command` and optional `filter`:

```yaml
hooks:
  on_status_change:
    - command: "notify.sh {id} {old} {new}"
      filter:
        new_status: "closed"
```

### Filter Keys

| Key | Matches Against |
|-----|----------------|
| `new_status` | `event.NewValue` |
| `old_status` | `event.OldValue` |
| `assignee` | `event.Actor` |

All filter conditions must match (AND logic). Empty filter matches all events.

### Template Variables

| Variable | Expands To |
|----------|-----------|
| `{id}` | Task ID |
| `{old}` | Previous value |
| `{new}` | New value |
| `{actor}` | Who performed the action |
| `{field}` | Which field changed |

### Execution

- Hooks run as `sh -c <command>` in background goroutines
- Errors are logged to `~/.slate/hooks.log`
- Hooks never block the main operation
- stdout and stderr are captured on failure

### Enable Hooks in SDK

```go
cfg, _ := slate.LoadConfig("")
store, _ := slate.Open(ctx, cfg.DBPath, slate.WithConfig(cfg))
slate.EnableHooks(store, cfg) // registers catch-all listener
```

## Notion Configuration

Notion config is stored separately in `~/.slate/notion.yaml` (0600 permissions) for security. See the [Notion Sync Guide](notion-sync.md) for full documentation.

| Path | Purpose |
|------|---------|
| `~/.slate/notion.yaml` | Notion API token, database ID, property/status/priority mappings |

**Key config sections:**
- `property_map` — maps Slate fields to Notion property names
- `status_map` — bidirectional status mapping (many-to-one for pull)
- `priority_map` / `priority_reverse` — priority mapping
- `dep_map` — dependency type → Notion relation property
- `auto_create_properties` — create missing Notion properties on first sync
- `rate_limit` — delay between API calls (default: 334ms ≈ 3 req/sec)

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `SLATE_HOME` | Override home directory | `~/.slate/` |
| `NO_COLOR` | Disable ANSI color output | (unset) |

## CLI Flag Precedence

```
CLI flag > config file > default
```

For example, `--tree` overrides `default_view: list` in config.
