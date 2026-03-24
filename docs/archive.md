# Task Archive

Move old closed tasks to a separate database to keep the main DB fast and clean.

## Archive Tasks

```bash
slate archive run                    # archive tasks closed 90+ days ago
slate archive run --before 2026-01-01  # archive before specific date
```

Tasks, comments, attributes, and checkpoints are moved to `~/.slate/slate-archive.db`. Events stay in the main DB for metrics continuity.

## Restore Tasks

```bash
slate archive restore                # restore all archived tasks
slate archive restore --task st-ab12 # restore specific task
```

## List Archived

```bash
slate archive list                   # list all archived tasks
slate archive list --json
```

## How It Works

1. Opens (or creates) the archive database with the same schema
2. Copies matching closed tasks + related data to archive
3. Deletes from main DB (events preserved for metrics)
4. Reversible via `slate archive restore`
