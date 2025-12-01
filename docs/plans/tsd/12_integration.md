## 12. Integration Patterns with Other Contexts

### 12.1 With Beads Issue Management

**Pattern:** Shared DB with Anti-Corruption Layer.

* The `SQLiteIssueRepositoryAdapter` :

  * Encapsulates raw SQL.
  * Maps DB records to domain `Issue`.
  * Builds predicates via `ReadySpecification` and `ClaimFilters`.
* All domain logic uses the domain model; no external code sees raw schemas.

**Considerations:**

* Must remain consistent with `bd ready` semantics:

  * Either re-use the same SQL view/function if exposed,
  * Or mirror its logic as closely as possible in `ReadySpecification`.

### 12.2 With Agents / Orchestrators

**Pattern:** Open Host Service (CLI).

* Agents treat `bd-claim` as a stateless “claim one issue” endpoint.
* Orchestrators can implement higher-level policies:

  * Round-robin agent creation,
  * Per-agent filtering strategies,
  * Quotas and fairness.

### 12.3 With Git / Workspace

* No direct integration.
* Indirect effects:

  * When issues are claimed, they become `in_progress` in DB and eventually in JSONL and Git via Beads daemon.
* `WorkspaceDiscoveryAdapter` ensures:

  * We only connect to valid `.beads` DB instances.
  * Avoids cross-repo confusion.
