## 11. Data Persistence Strategy

### 11.1 Storage Technology

* **SQLite** (the existing Beads DB):

  * Single file within `.beads` (e.g., `.beads/beads.db`).
  * Mode: WAL or as configured by Beads.
  * Shared with `bd` CLI and Beads daemon.

### 11.2 Schema Usage

We strictly operate on the following tables defined in `beads` (v0.27.0+):

*   **`issues`**
    *   `id` (TEXT PK)
    *   `status` (TEXT)
    *   `priority` (INTEGER)
    *   `assignee` (TEXT)
    *   `content_hash` (TEXT) - *Must be re-calculated on update*
    *   `external_ref` (TEXT)
    *   `source_repo` (TEXT)
    *   `compaction_level` (INTEGER)
    *   `compacted_at` (DATETIME)
    *   `original_size` (INTEGER)
*   **`blocked_issues_cache`**
    *   `issue_id` (TEXT PK)
    *   *Critical:* We must respect this cache for the "Ready" predicate.
*   **`dependencies`** (Read-only for claims)
    *   `issue_id`, `depends_on_id`, `type`
*   **`labels`**
    *   `issue_id`, `label`

**Views:**
*   **`ready_issues`** (Reference implementation, though we use the cache table for performance).

If new columns are added to these tables, we ignore them unless they affect the hash calculation. If critical columns (`status`, `assignee`) are missing or renamed, we emit `SCHEMA_INCOMPATIBLE`.

### 11.3 Transactions & Locking

**Transaction lifecycle:**

1. `BEGIN IMMEDIATE;`

   * Acquire a reserved lock early to prevent lock upgrade issues.
2. Execute `UPDATE … WHERE (SELECT … LIMIT 1)` as described earlier.
3. If 0 rows updated:

   * Optionally retry (limited) if due to race (`SQLITE_BUSY` or lost candidate).
   * Else treat as `NoIssueAvailable`.
4. `COMMIT;` on success, `ROLLBACK;` on error.

**Busy Timeout & Retry Policy:**

* Default `busy_timeout` (e.g., 1000–3000ms) set per connection.
* If `SQLITE_BUSY`:

  * Exponential backoff with jitter: e.g., 20ms, 40ms, 80ms up to configured maximum attempts (3–5).
  * On persistent contention:

    * Emit `ClaimFailed` with code `SQLITE_BUSY`.
    * Return error JSON.

### 11.4 Schema Evolution

* Adapter must be robust to minor schema changes:

  * Use `PRAGMA table_info(issues)` to discover column existence.
  * If new columns are added, we ignore them.
  * If critical columns (`status`, `assignee`) are missing or renamed:

    * Emit `SCHEMA_INCOMPATIBLE` error.
* No migrations are run by `bd-claim`; schema ownership remains with Beads.
