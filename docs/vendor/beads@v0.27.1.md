# Specification & Migration Plan: Beads v0.27.1

## 1. Version Metadata

*   **Beads Version:** v0.27.1
*   **Beads Commit:** 5413536d5c515f9dbf4c230e73b7b2e40f7dc811
*   **Source:** `vendor/beads` (vendored source code)

## 2. Database Schema Specification

The following schema is derived from `vendor/beads/internal/storage/sqlite/schema.go` and `migrations/`.

### Tables

*   **`issues`**
    *   `id` TEXT PRIMARY KEY
    *   `content_hash` TEXT
    *   `title` TEXT NOT NULL (length <= 500)
    *   `description` TEXT DEFAULT ''
    *   `design` TEXT DEFAULT ''
    *   `acceptance_criteria` TEXT DEFAULT ''
    *   `notes` TEXT DEFAULT ''
    *   `status` TEXT DEFAULT 'open' (CHECK: `open`, `in_progress`, `blocked`, `closed`)
    *   `priority` INTEGER DEFAULT 2 (0-4)
    *   `issue_type` TEXT DEFAULT 'task'
    *   `assignee` TEXT
    *   `estimated_minutes` INTEGER
    *   `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP
    *   `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP
    *   `closed_at` DATETIME (CHECK: set iff status='closed')
    *   `external_ref` TEXT (Indexed, Unique where not null)
    *   `compaction_level` INTEGER DEFAULT 0
    *   `compacted_at` DATETIME
    *   `compacted_at_commit` TEXT
    *   `original_size` INTEGER
    *   `source_repo` TEXT DEFAULT '.' (Indexed)

*   **`dependencies`**
    *   `issue_id` TEXT
    *   `depends_on_id` TEXT
    *   `type` TEXT DEFAULT 'blocks' ('blocks', 'parent-child', 'related', 'discovered-from')
    *   `created_at` DATETIME
    *   `created_by` TEXT
    *   PK: (`issue_id`, `depends_on_id`)

*   **`labels`**
    *   `issue_id` TEXT
    *   `label` TEXT
    *   PK: (`issue_id`, `label`)

*   **`comments`**
    *   `id` INTEGER PK AUTOINCREMENT
    *   `issue_id` TEXT
    *   `author` TEXT
    *   `text` TEXT
    *   `created_at` DATETIME

*   **`events`** (Audit Trail)
    *   `id` INTEGER PK AUTOINCREMENT
    *   `issue_id` TEXT
    *   `event_type` TEXT
    *   `actor` TEXT
    *   `old_value` TEXT
    *   `new_value` TEXT
    *   `comment` TEXT
    *   `created_at` DATETIME

*   **`config`**
    *   `key` TEXT PK
    *   `value` TEXT

*   **`metadata`**
    *   `key` TEXT PK
    *   `value` TEXT

*   **`dirty_issues`** (Incremental Export)
    *   `issue_id` TEXT PK
    *   `marked_at` DATETIME
    *   `content_hash` TEXT

*   **`export_hashes`** (Dedup)
    *   `issue_id` TEXT PK
    *   `content_hash` TEXT
    *   `exported_at` DATETIME

*   **`child_counters`** (ID Generation)
    *   `parent_id` TEXT PK
    *   `last_child` INTEGER

*   **`issue_snapshots`** (Compaction)
    *   `id` INTEGER PK AUTOINCREMENT
    *   `issue_id` TEXT
    *   `snapshot_time` DATETIME
    *   `compaction_level` INTEGER
    *   `original_size` INTEGER
    *   `compressed_size` INTEGER
    *   `original_content` TEXT
    *   `archived_events` TEXT

*   **`compaction_snapshots`** (Restoration)
    *   `id` INTEGER PK AUTOINCREMENT
    *   `issue_id` TEXT
    *   `compaction_level` INTEGER
    *   `snapshot_json` BLOB
    *   `created_at` DATETIME

*   **`repo_mtimes`** (Multi-repo Optimization)
    *   `repo_path` TEXT PK
    *   `jsonl_path` TEXT
    *   `mtime_ns` INTEGER
    *   `last_checked` DATETIME

*   **`blocked_issues_cache`** (Performance Cache - **CRITICAL**)
    *   `issue_id` TEXT PK
    *   *Note:* Contains IDs of all issues that are transitively blocked. Used to speed up `Ready` queries.

### Views

*   `ready_issues`: Recursive CTE view (Legacy/Reference? The code now prefers `blocked_issues_cache`).
*   `blocked_issues`: Aggregates blockers.

## 3. The "Ready Predicate" Definition

### Conceptual Definition
"Ready Work" consists of issues that:
1.  Are actionable (Status is `open` or `in_progress`).
    *   *Note:* `bd-claim` specifically targets `open` issues for claiming, but `bd ready` shows `in_progress` too.
2.  Have **no open blocking dependencies** (transitive).
3.  Satisfy user filters (Assignee, Priority, Labels).

### Technical Definition (from `vendor/beads/internal/storage/sqlite/ready.go`)

```go
// WHERE clause construction
whereClauses := []string{}
// Status defaults to open OR in_progress
whereClauses = append(whereClauses, "i.status IN ('open', 'in_progress')")

// The critical blocking check using the cache table
// NOT EXISTS (SELECT 1 FROM blocked_issues_cache WHERE issue_id = i.id)
query := fmt.Sprintf(`
    SELECT ...
    FROM issues i
    WHERE %s
    AND NOT EXISTS (
      SELECT 1 FROM blocked_issues_cache WHERE issue_id = i.id
    )
    %s
    %s
`, whereSQL, orderBySQL, limitSQL)
```

**Crucial Change:** The "Ready" logic now relies on `blocked_issues_cache` table for performance (bd-5qim), replacing the recursive CTE in the read path. `bd-claim` **must** respect this cache table to be consistent with `bd`.
*Observation:* The current SDD (`docs/plans/SDD.md`) already correctly reflects this requirement.

## 4. CLI Impact Analysis

### `bd-claim` Implementation Requirements

1.  **Query Strategy:**
    *   MUST NOT use recursive CTEs for blocking checks if `blocked_issues_cache` exists.
    *   MUST check for existence of `blocked_issues_cache` (it's a migration, might not exist in older DBs, but `bd` v0.27+ guarantees it).
    *   The "Claim" operation is:
        ```sql
        UPDATE issues
        SET status = 'in_progress', assignee = ?, updated_at = CURRENT_TIMESTAMP
        WHERE id = (
            SELECT id FROM issues i
            WHERE status = 'open'
            AND NOT EXISTS (SELECT 1 FROM blocked_issues_cache WHERE issue_id = i.id)
            -- Apply other filters (priority, labels, unassigned)
            ORDER BY ...
            LIMIT 1
        )
        RETURNING *
        ```

2.  **Type Mirroring:**
    *   `Issue` struct needs:
        *   `SourceRepo` (string)
        *   `CompactionLevel` (int)
        *   `CompactedAt` (time)
        *   `CompactedAtCommit` (string)
        *   `OriginalSize` (int)
        *   `ContentHash` (string)
    *   `blocked_issues_cache` management is done by `beads` daemon/CLI. `bd-claim` should generally **read** from it.
        *   *Risk:* If `bd-claim` updates a dependency, it might need to invalidate cache. But `bd-claim` only *claims* (updates status/assignee).
        *   *Invariant:* Does changing status from `open` -> `in_progress` affect blocking?
            *   Blockers block if they are `open`, `in_progress`, or `blocked`.
            *   So `open` -> `in_progress` does **not** unblock anything.
            *   Thus, `bd-claim` **does not need to invalidate/update the blocked cache** when claiming an issue.

3.  **Migration Responsibility:**
    *   `bd-claim` is a consumer of the schema, not the owner.
    *   It MUST NOT run migrations (`EnsureSchema`, etc.) on startup.
    *   It should fail fast with a clear error if the schema is incompatible (e.g., missing tables).

## 5. Migration Todos

*   [ ] **Update Domain Models:** Sync domain models with new fields from Beads `Issue` (SourceRepo, Compaction, etc.).
*   [ ] **Update Repository Logic:**
    *   Ensure `ClaimReadyIssue` uses `blocked_issues_cache` in its selection SQL.
    *   Verify `blocked_issues_cache` is populated (or fallback/error if missing - though migration `015` ensures it).
*   [ ] **Review SQL Queries:**
    *   Check `ORDER BY` clauses match Beads' "Hybrid" sort policy if default ordering is desired.
    *   Ensure `updated_at` is touched on claim.
*   [ ] **Verify Tables:** Ensure all read/write operations tolerate the existence of new columns (`source_repo`, `compaction_*`).
*   [ ] **Tests:** Update and add tests to ensure **100% code coverage** for all new and modified logic.

