## 7. Key Aggregates, Entities, Value Objects

### 7.1 Aggregates

**Aggregate: Issue**

* **Identity:** `IssueId`

* **Root:** `Issue`

* **Fields (as seen in this context, mapped from Beads):**

  * `id: IssueId`
  * `status: IssueStatus` (`open`, `in_progress`, `closed`, …)
  * `assignee: Option<AgentName>`
  * `priority: Priority` (e.g., `low`, `medium`, `high`, numeric weights)
  * `labels: LabelSet`
  * `blocked: bool` (derived from `blocked_issues_cache` existence)
  * `source_repo: String` (path to source repository)
  * `content_hash: String`
  * `created_at: Timestamp`
  * `updated_at: Timestamp`
  * `compaction_level`: Int (opaque, preserved)

* **Invariants (within this context):**

  * A claim can only be applied if:

    * `status == open`
    * NOT blocked (`blocked == false` or equivalent)
    * `assignee` matches filter (e.g., unassigned or allowed pattern)
  * Claim transition:

    * Before: `status = open`, `assignee = any` (subject to filters)
    * After: `status = in_progress`, `assignee = AgentName(agent)`

* **Domain Behavior:**

  * `Issue.claim(agent: AgentName) -> IssueClaimed | ClaimRejected`

    * Validates invariants.
    * Returns domain event describing transition.
    * In practice, due to concurrent access, most invariants are enforced in persistence layer via `UPDATE … WHERE predicate`.

> Note: In this context, we do **not** own the lifecycle transitions beyond `open → in_progress`. Other transitions are managed by Beads Issue Management.

### 7.2 Entities

There are **no separate non-root entities** within this aggregate in this context; any substructures (e.g., checklist items) are opaque to Issue Claiming and not used in predicates.

### 7.3 Value Objects

1. **IssueId**

   * Underlying: string/UUID/int as provided by Beads.
   * Rules:

     * Opaque; no semantic operations beyond equality.

2. **AgentName**

   * Underlying: string.
   * Rules:

     * Non-empty, max length constraint (e.g., 64 chars).
     * Sanitized for logging (no control chars).

3. **IssueStatus**

   * Enumerated type: `open`, `in_progress`, `closed`, `blocked`, `archived`, etc.
   * Mapped from Beads’ status column.
   * Used for claim predicate and invariants.

4. **Priority**

   * Enumerated or numeric.
   * Used for ordering candidates and filtering.

5. **LabelSet**

   * Collection of label strings.
   * Provides methods like `contains(label)` and `matchesAny(labels)`.

6. **ClaimFilters**

   * Immutable object representing all filter options:

     * `only_unassigned: bool`
     * `include_labels: Set<String>`
     * `exclude_labels: Set<String>`
     * `min_priority: Option<Priority>`
   * Used by the selector to build predicates.

7. **ReadySpecification**

   * Encapsulates “ready” logic:

     * `status == open`
     * `not blocked`
     * Additional constraints aligned with `bd ready`.
   * Provides:

     * `sql_predicate_fragment()` for the repository.
     * `isSatisfiedBy(issue: Issue)` for defensive checks.
