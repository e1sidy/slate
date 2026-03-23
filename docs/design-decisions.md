# Design Decisions

Every major design choice, what was considered, and why we chose what we did.

## 1. Pure-Go SQLite (No CGO)

**Choice:** `modernc.org/sqlite`
**Rejected:** `mattn/go-sqlite3` (CGO), Dolt, PostgreSQL
**Why:** Single binary output, zero build dependencies, cross-compilation works. Slight performance cost (~2x vs CGO SQLite) is acceptable for a local task tracker. Distribution simplicity far outweighs raw speed.

## 2. Database Transactions for Multi-Step Operations

**Choice:** `CloseTask` and `CancelTask` wrap all SQL in `db.BeginTx()`
**Rejected:** Individual statements without transactions
**Why:** A crash between closing a task and auto-unblocking its dependents would leave the database in an inconsistent state. Transactions ensure all-or-nothing semantics. This is the single most important architectural decision.

## 3. Atomic Claim Locking

**Choice:** `UPDATE ... WHERE (assignee IS NULL OR assignee = '' OR assignee = ?)`
**Rejected:** Unconditional `UPDATE SET assignee = ?`
**Why:** Without the WHERE clause, two agents claiming the same task simultaneously would both succeed — the second silently overwrites the first. With atomic locking, the second agent gets `ErrAlreadyClaimed`.

## 4. Lease-Based Claim Expiry

**Choice:** `ExpireLeases()` auto-releases claims older than a configurable timeout
**Rejected:** Manual release only
**Why:** When an AI agent crashes mid-task, its claim would be held forever. Lease expiry ensures zombie claims are automatically released, making tasks available for other agents.

## 5. `context.Context` on Every Method

**Choice:** First parameter on all public SDK functions
**Rejected:** No context
**Why:** Enables cancellation, timeouts, and proper HTTP server integration. Without context, long-running queries can't be cancelled, and the SDK can't be embedded in a web server that needs request-scoped deadlines.

## 6. Mutable Comments

**Choice:** Comments support edit and delete
**Rejected:** Immutable comments (append-only)
**Why:** Users need to fix typos. Agents write structured comments that may need correction. The audit trail (events table) preserves history even after edits.

## 7. Full JSONL Export

**Choice:** Export tasks + comments + dependencies + attributes + checkpoints
**Rejected:** Task-only export
**Why:** A task-only export loses critical data on migration. If you move to a new machine and import, you'd lose all comments, dependency relationships, custom attributes, and progress checkpoints. Full export enables lossless backup and migration.

## 8. No Web UI

**Choice:** No embedded web interface
**Rejected:** Embedded kanban board
**Why:** Notion will serve as the human interface (Phase 3). Building and maintaining a web UI adds authentication concerns, frontend dependencies, CSS maintenance, and security surface area (XSS, CSRF). Notion already has kanban, timeline, calendar, and gallery views.

## 9. LIKE Search Initially, FTS5 Later

**Choice:** `LIKE '%query%'` for search
**Rejected:** FTS5 from day one
**Why:** Ship fast. LIKE search works for small-to-medium task databases. FTS5 requires additional schema (virtual tables, triggers) and testing. It's planned for Phase 5 when the ecosystem matures.

## 10. CI-Tested JSON Output

**Choice:** Every `--json` command is tested in CI to produce valid JSON
**Rejected:** Trust-based approach (hope it works)
**Why:** Machine-parseable output is critical for AI agents. If `--json` silently outputs human-readable text (which has happened in other tools), agents parse garbage and fail silently. CI tests catch regressions immediately.

## 11. `st-` ID Prefix

**Choice:** Default prefix `st-`
**Rejected:** `sl-`
**Why:** `st-` reads better and avoids confusion with `sl` (a common Unix command for screen lock). The prefix is configurable via `slate config set prefix myapp`.

## 12. Hierarchical Ladder IDs

**Choice:** Task IDs like `st-a3f8.1.1` encode hierarchy
**Rejected:** Opaque auto-increment IDs
**Why:** Hierarchy is immediately visible from the ID. You can infer parent-child relationships without querying the database. `st-a3f8.1` is clearly a child of `st-a3f8`.
