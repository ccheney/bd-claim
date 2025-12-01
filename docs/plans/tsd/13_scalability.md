## 13. Scalability, Performance & Reliability Strategy

### 13.1 Scalability (Local Swarm)

Target: **5–10 concurrent agents** per workspace, up to thousands of issues.

Strategies:

1. **Efficient SQL**

   * Indexes:

     * Ensure indexes on `(status, priority, created_at)` or similar.
     * If not guaranteed, `bd-claim` may log a warning but will still operate.
   * Use `LIMIT 1` with sensible order to avoid scanning.

2. **Short-Lived Connections**

   * Each invocation opens a connection, performs a single transaction, and closes.
   * Minimizes long-lived lock holders.

3. **Backoff on Contention**

   * Exponential backoff to avoid thundering herds when many agents start simultaneously.

4. **Configurable Timeouts**

   * `--timeout-ms` allows tuning per environment.

### 13.2 Performance

* Expected latency per claim:

  * Typical: < 50–100ms on dev machine.
* Optimizations:

  * Prepared statements reused across invocations when runtime allows (e.g., if we implement a library mode).
  * Avoid unnecessary data loading:

    * Only columns needed for predicates and returned payloads.

### 13.3 Reliability

* **Idempotency:**

  * A claim success means the DB **already reflects** claimed state.
  * If agent crashes immediately after receiving response:

    * The issue remains `in_progress` and will not be automatically re-queued (this is consistent with Beads semantics and left to human/orchestrator).

* **Failure Modes:**

  * DB missing ⇒ clear `DB_NOT_FOUND` error.
  * Workspace missing ⇒ `WORKSPACE_NOT_FOUND`.
  * Lock contention ⇒ `SQLITE_BUSY` (after retries).
  * Unexpected SQL errors ⇒ `UNEXPECTED`.

* **Defensive Programming:**

  * Validate agent name early.
  * Validate filters.
  * Distinguish “no candidate found” (`NoIssueAvailable`) from technical failure (`ClaimFailed`).

* **Circuit Breakers (Optional)**

  * If we embed `bd-claim` in a long-running orchestrator, a simple circuit breaker can:

    * Stop calling `bd-claim` when repeated `SQLITE_BUSY` or `DB_NOT_FOUND` errors occur,
    * Require manual intervention.
