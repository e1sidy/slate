# FTS5 Full-Text Search

Slate uses SQLite FTS5 for fast, ranked full-text search across task titles, descriptions, and notes.

## Usage

```bash
slate search "authentication"       # FTS5 search (word-level, ranked)
```

FTS5 supports:
- **Word matching**: `"auth"` matches "authentication", "authorized"
- **Phrase search**: `"fix auth"` matches the exact phrase
- **Boolean**: `auth NOT session`
- **Prefix**: `auth*` matches any word starting with "auth"

## SDK

```go
results, err := store.SearchFTS(ctx, "JWT tokens")
```

Falls back to LIKE-based search if FTS5 is unavailable.

## Rebuild Index

After bulk imports, rebuild the FTS index:

```bash
# Automatic: slate import rebuilds automatically
# Manual SDK call:
store.RebuildFTSIndex(ctx)
```

## How It Works

Migration v5 creates an FTS5 virtual table `tasks_fts` with triggers that keep it in sync with the `tasks` table on insert, update, and delete.
