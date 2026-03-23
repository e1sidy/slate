# Getting Started

## Installation

```bash
# Via Go
go install github.com/e1sidy/slate/cmd/slate@latest

# Or build from source
git clone https://github.com/e1sidy/slate.git
cd slate && go build -o slate ./cmd/slate/
```

Requirements: Go 1.22+

## Initialize

```bash
slate config init --prefix st
```

This creates `~/.slate/` with a default config and database.

## Create Your First Task

```bash
slate create "Fix login bug" --type bug --priority 1
# Created st-a3f8: Fix login bug

slate create "Add OAuth support" --type feature --priority 2
# Created st-b2c1: Add OAuth support
```

## List Tasks

```bash
slate list
# [ ] P1 Fix login bug st-a3f8
# [ ] P2 Add OAuth support st-b2c1

# JSON output for scripting
slate list --json
```

## Show Details

```bash
slate show st-a3f8
# [ ] P1 Fix login bug st-a3f8
#   Type: bug
#   Created: 2026-03-23 14:30
#   Updated: 2026-03-23 14:30
```

## Task Hierarchy

Create an epic with subtasks:

```bash
slate create "API v2" --type epic
# Created st-c4d5: API v2

slate create "Design endpoints" --parent st-c4d5
# Created st-c4d5.1: Design endpoints

slate create "Implement handlers" --parent st-c4d5
# Created st-c4d5.2: Implement handlers

slate list --tree
# [ ] P0 API v2 st-c4d5
# [ ] P0 Design endpoints st-c4d5.1
# [ ] P0 Implement handlers st-c4d5.2
```

## Dependencies

```bash
# st-c4d5.2 depends on st-c4d5.1 (implement after design)
slate dep add st-c4d5.2 st-c4d5.1

# What's ready to work on?
slate ready
# [ ] P1 Fix login bug st-a3f8
# [ ] P2 Add OAuth support st-b2c1
# [ ] P0 Design endpoints st-c4d5.1
```

Note: `st-c4d5.2` is not listed because it's blocked by `st-c4d5.1`.

## Claim a Task

```bash
slate update st-a3f8 --claim --actor agent-1
# Claimed st-a3f8 by agent-1

slate list
# [>] P1 Fix login bug st-a3f8 @agent-1
# [ ] P2 Add OAuth support st-b2c1
```

## Add Comments and Checkpoints

```bash
# Comments
slate comment add st-a3f8 "Found the root cause in auth middleware"

# Structured checkpoint
slate checkpoint add st-a3f8 \
  --done "Fixed token validation" \
  --decisions "Used JWT instead of sessions" \
  --next "Add integration tests"
```

## Close a Task

```bash
slate close st-a3f8 --reason "Fixed in commit abc123"
# Closed st-a3f8
```

If the task had dependents marked as `blocked`, they'll automatically transition to `open`.

## Custom Attributes

```bash
# Define an attribute type
slate attr define env string --desc "Deployment environment"

# Set on a task
slate attr set st-b2c1 env "staging"

# Read it back
slate attr get st-b2c1 env
# env = staging (string)
```

## Export and Import

```bash
# Full export (tasks + comments + deps + attrs + checkpoints)
slate export --file backup.jsonl

# Import into another instance
slate import backup.jsonl
```

## Metrics

```bash
slate metrics
# Tasks created:    5
# Tasks closed:     1
# Currently open:   4

slate next
# Recommended: [ ] P0 Design endpoints st-c4d5.1
```

## Health Check

```bash
slate doctor
# ✓ integrity: database integrity OK
# ✓ orphaned_parents: no orphaned parent references
# ✓ cycles: no dependency cycles
# ✓ task_summary: total=5 open=4 in_progress=0 closed=1
# All checks passed.
```

## Shell Completions

```bash
# Bash
source <(slate completion bash)

# Zsh
source <(slate completion zsh)

# Fish
slate completion fish | source
```

## Next Steps

- [CLI Reference](cli-reference.md) — every command and flag
- [SDK Reference](sdk-reference.md) — embed Slate in your Go app
- [Concepts](concepts.md) — status model, hierarchy, dependencies
- [Configuration](configuration.md) — hooks, config file, env vars
