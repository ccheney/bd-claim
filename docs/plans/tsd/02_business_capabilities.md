## 2. Business Capabilities & Key Use Cases

### 2.1 Business Capabilities

This bounded context owns the following capabilities:

1. **Atomic Issue Claiming**

   * Select exactly one “ready” issue according to Beads semantics.
   * Transition it to a claimed state (`status = in_progress`, `assignee = <agent>`) atomically.
   * Guarantee: in a single Beads SQLite DB, **no two successful claims** return the same issue.

2. **Claim Filtering & Routing**

   * Restrict which issues are eligible for claim by:

     * Assignee (e.g., unassigned only),
     * Labels,
     * Priority,
     * Optional predicate extensions (future: components, tags, etc.).
   * Provide a simple mechanism for orchestrators to “shape” which work an agent can see without encoding DB details.

3. **Ready Predicate Compatibility**

   * Mirror/complement `bd ready` predicate:

     * Only open issues,
     * No blockers (checked via `blocked_issues_cache` for performance),
     * Not already in-progress.
   * Abstract the ready check behind a specification so internal logic can evolve while preserving contract.

4. **Stable Agent-Facing Contract**

   * Simple JSON-first CLI interface suitable for:

     * Direct agent usage,
     * Orchestrator scripts,
     * Tool integrations (e.g., AI coding environments).
   * Clear success vs. “no work” vs. error signaling.

5. **Concurrency-Safe DB Interaction**

   * Use SQLite transactions and locking in a way that:

     * Does not corrupt the Beads DB,
     * Minimizes lock contention with other `bd` and Beads daemon operations,
     * Avoids deadlocks and pathological lock storms.

6. **Operational Introspection**

   * Provide **inspectable**, human-runnable CLI behavior:

     * Humans can run `bd-claim` to debug swarm behavior,
     * Structured logs and error messages.

### 2.2 Key Use Cases

1. **UC-1: Single Agent Claims Work**

   * Trigger: Agent is idle and wants a task.
   * Flow:

     1. Agent runs `bd-claim --agent backend-1 --json`.
     2. System picks a ready issue, sets `status = in_progress`, `assignee = backend-1`.
     3. Returns issue in JSON.
   * Outcome: Agent gets exclusive ownership of that issue.

2. **UC-2: Multiple Agents Racing for Work**

   * Trigger: Multiple agents start concurrently and all call `bd-claim`.
   * Flow:

     1. Each invocation tries to atomically claim **one** ready issue.
     2. SQLite locking ensures serialized updates; losing contenders see zero updated rows and must retry or return `issue: null`.
   * Outcome: No duplicate claims; each claimed issue is assigned to exactly one agent.

3. **UC-3: No Work Available**

   * Trigger: Agent calls `bd-claim --agent test-agent`.
   * Flow:

     1. Query candidate ready issues with filters.
     2. None are available or claimable.
   * Outcome: Return `{ status: "ok", agent: "test-agent", issue: null }`.

4. **UC-4: Filtered Claim**

   * Trigger: Specialized agent that only wants certain labels.
   * Flow:

     1. Agent calls `bd-claim --agent infra-1 --label infra --priority high`.
     2. Only ready issues matching those filters are considered.
   * Outcome: Agent either gets a filtered issue or `issue: null`.

5. **UC-5: Human Debugging / Inspection**

   * Trigger: Overseer wants to verify claim behavior.
   * Flow:

     1. Run `bd-claim --agent debug-agent --json --dry-run`.
     2. See which issue would be claimed without committing mutations.
   * Outcome: Safe introspection of selection logic.
