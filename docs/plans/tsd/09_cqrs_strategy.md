## 9. Read Model / Query Strategy (CQRS)

### 9.1 CQRS Approach

Given the small scope and single operation, we adopt a **lightweight CQRS** pattern:

* **Command Side:**

  * Operation: `ClaimIssueCommand(agent, filters)`.
  * Implemented via:

    * `ClaimIssueUseCase` → `ClaimIssueService` → `IssueRepositoryPort`.

* **Query Side:**

  * We do not maintain a separate read model DB.
  * Queries operate directly against Beads’ SQLite, using:

    * `ReadySpecification` + `ClaimFilters` to build SQL predicates.

For observability/debugging, we may optionally expose a **read-only** mode (`--dry-run`) that:

* Executes the selection logic **without** the state change,
* Returns the candidate issue as read from DB without committing an update.

### 9.2 Selection Strategy

We must avoid scanning the entire issue set under contention while still preserving fairness where possible.

**Candidate Selection Rules:**

1. Apply ready predicate:

   * `status == 'open'`
   * `not blocked`
2. Apply filters:

   * `assignee IS NULL` if `only_unassigned`,
   * `labels @> include_labels` (or equivalent),
   * `labels !&& exclude_labels` (or equivalent),
   * `priority >= min_priority`.
3. Order candidates:

   * Default ordering:

     * `ORDER BY priority DESC, created_at ASC, id ASC`
   * Allows:

     * Prioritization of more important tasks,
     * Approximate FIFO within priority.

**Atomic Update Pattern (Command Side):**

Within a single transaction:

```sql
UPDATE issues
SET status = 'in_progress',
    assignee = :agent,
    updated_at = :now
WHERE id = (
  SELECT i.id
  FROM issues i
  WHERE i.status = 'open'
    AND NOT EXISTS (SELECT 1 FROM blocked_issues_cache bic WHERE bic.issue_id = i.id)
    -- AND filters (assignee, labels, etc.)
  ORDER BY i.priority DESC, i.created_at ASC, i.id ASC
  LIMIT 1
)
AND status = 'open'
-- optional: AND assignee IS NULL or matches filter
RETURNING *;
```

* If a row is returned:

  * Command success: we have a claimed issue.
* If zero rows:

  * Either no candidate existed or we lost a race; treat as `NoIssueAvailable` (optionally with retry).

This pattern inherently combines **read + write** in one round trip, preventing classic “select-then-update” race conditions.
