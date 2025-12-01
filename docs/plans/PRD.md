# Product Requirements Document (PRD) — `bd-claim`

## 1. Context & Scope

Beads provides a powerful **task graph + issue tracker** for agents and humans, with:

* JSONL issues committed to git as the source of truth,
* A local SQLite cache for fast queries and updates,
* Auto-sync between DB ↔ JSONL ↔ Git.

Beads is explicitly designed for **agents**, but when you run **multiple agents in parallel on the same repo**, they can race to claim the same “ready” task if all they see is `bd ready` output.

`bd-claim` is a **small companion CLI** that sits in front of Beads and gives you a **safe, atomic “claim a ready issue” primitive**.

### In-scope

* Single-repo, single-machine scenario (“Local Swarm”):

  * Multiple agents running concurrently in the same working tree.
  * Single `.beads` workspace with one Beads SQLite DB.
* A single CLI command (`bd-claim`) that:

  * Exposes an **atomic claim** operation (find → update in one DB transaction).
  * Is **agent-facing** (JSON-first interface).
* No extra servers:

  * `bd-claim` is invoked as a normal CLI, like `bd`.
* No new long-lived data structures:

  * Uses Beads’ existing issue schema (`status`, `assignee`, etc.).
  * No new top-level tables required.

### Out-of-scope

* Multi-repo or cross-repo task claiming.
* Multi-machine “global claim” guarantee (Beads itself is eventually consistent via Git; `bd-claim` only guarantees local DB).
* Replacement of `bd ready`, `bd update`, etc.—it is a *supplement*, not a replacement.
* Messaging, file leases, or agent chat (those belong to higher-level tools like “agent mail”, not `bd-claim`).

---

## 2. Goals & Non-goals

### 2.1. Goals

1. **Prevent double-claims inside a single workspace**
   When multiple agents call `bd-claim` concurrently, at most **one** should successfully claim any given ready issue.

2. **Stable, simple agent contract**
   Agents should have a single, easy-to-remember flow:

   * “When I need a task, call `bd-claim`. If it returns an issue, work that issue. If not, there’s no work.”

3. **Beads-native, no schema forks**
   Use Beads’ **existing fields**:

   * `status` (`open`, `in_progress`, `closed`, etc.),
   * `assignee`, priority, labels.

   Do **not** invent parallel bookkeeping structures.

4. **Safe under load**
   Under high concurrency (e.g. 5–10 agents on one workspace), `bd-claim` should:

   * Not cause SQLite lock storms,
   * Not corrupt the DB,
   * Respect the Beads daemon’s locking patterns (if daemon is running).

5. **Minimal operator overhead**
   Humans (overseers) don’t need to use `bd-claim` directly; they can keep using `bd`. `bd-claim` is primarily for agents and orchestrators, but should be inspectable/ad-hoc runnable.

### 2.2. Non-goals

* **No global scheduling policy.**
  `bd-claim` focuses on the *atomicity* of claiming. It won’t implement complex scheduling (fairness, quotas, specialization) beyond simple filters; those can live in agent/orchestrator logic on top.

* **No task lifecycle management**
  It does not mark tasks as done or spawn follow-up tasks; that remains the job of agents using `bd` and Beads semantics.

* **No replacement for Beads daemon or monitor**
  `bd-claim` isn’t a daemon or UI; the Beads daemon and optional monitor-webui remain the primary long-lived components for Beads operations.

---

## 3. Personas & Use Cases

### Persona A — Coding Agent

* AI coding agent running in a tool like Claude Code, Gemini CLI, etc.
* Needs a consistent way to **grab a task** and work on it.
* Uses:

  * `bd-claim` to get work,
  * `bd` CLI to inspect/update tasks,
  * Git to commit changes.

**Core use case:**

* “Give me the next ready task I can work on, without anyone else trampling it.”

### Persona B — Orchestrator (Agent supervisor)

* A script or meta-agent that spins up multiple worker agents.
* Needs to avoid writing complicated race-handling logic.
* Delegates “atomic claim” to `bd-claim`, so each worker’s startup sequence is simple.

### Persona C — Human Overseer

* Developer overseeing the swarm.
* Interacts with tasks via `bd` directly.
* Gains:

  * Confidence that tasks aren’t being double-claimed,
  * Simpler AGENTS.md instructions.

---

## 4. User Flows

### 4.1. Local Swarm: Single agent claim

1. Agent `backend-1` is idle.

2. It runs:

   ```bash
   bd-claim --agent backend-1 --json
   ```

3. `bd-claim`:

   * Connects to `.beads/beads.db`,
   * Starts a transaction,
   * Finds a ready issue,
   * Sets `status=in_progress`, `assignee="backend-1"`,
   * Commits.

4. Output returns the claimed issue.

5. Agent begins work on that issue using its ID.

### 4.2. Local Swarm: Two agents racing

1. Agents `backend-1` and `backend-2` both start at the same time.
2. Both call `bd-claim --agent <name> --json` concurrently.
3. Under the hood:

   * One transaction “wins” and commits status/assignee changes first.
   * The other transaction’s attempt to update that issue sees zero affected rows (no longer matches claimable predicate) and must either try another candidate or return `issue:null`.
4. Outcome:

   * `backend-1` gets issue `bd-7f3a`,
   * `backend-2` either gets a *different* issue or none at all.

### 4.3. No available work

1. Agent calls `bd-claim --agent test-agent`.

2. All ready issues are either:

   * Already `in_progress`, or
   * Filtered out, or
   * None exist.

3. `bd-claim` returns:

   ```json
   { "status":"ok", "agent":"test-agent", "issue":null }
   ```

4. Agent knows to stop/idle.

---

## 5. Functional Requirements

**FR-1: Single-issue claim**
`bd-claim` MUST claim **at most one** issue per invocation.

**FR-2: Idempotent success**
If it returns an `issue` in JSON, that issue’s state in Beads MUST already reflect:

* `status = in_progress` (or equivalent “claimed” status),
* `assignee = <agent-name>`.

**FR-3: No duplicate claims (local)**
Within a single Beads DB, two concurrent `bd-claim` runs MUST NOT both report the same issue as “successfully claimed” for different agents.

**FR-4: JSON-first output**
Default output MUST be JSON, with:

* `status: "ok" | "error"`,
* `agent` field,
* `issue` either an object or `null`,
* `error` and `message` on failure.

**FR-5: Filters**
CLI MUST support at least a basic subset of filters:

* by assignee (e.g. unassigned only),
* by labels and/or priority.

These filter the set of ready issues considered for claiming.

**FR-6: Ready predicate compatibility**
The internal “ready” predicate MUST match what `bd ready` defines:

* Open issues,
* No blockers,
* Not already in progress.

**FR-7: No schema fork**
`bd-claim` MUST use existing Beads issue fields and not require schema changes.

---

## 6. Non-functional Requirements

**NFR-1: Performance**
Typical claim operation should complete in <100ms on a normal dev machine for a small/medium issue set (matching Beads’ “fast local operations” claim).

**NFR-2: Concurrency safety**
Under multiple concurrent invocations (e.g., up to 10 agents), `bd-claim` MUST avoid DB corruption or lock contention crashes. It should honor the same locking patterns as the Beads daemon/CLI.

**NFR-3: Robust failure reporting**
Failures (DB not found, Beads not initialized, invalid flags) MUST be explicitly surfaced with clear error codes and messages.

**NFR-4: Zero config**
`bd-claim` MUST auto-discover the Beads DB in the current repo (similar to `bd` behavior) and not require manual DB path config in normal use.

**NFR-5: Quality Assurance**
The codebase MUST maintain **100% test coverage** to ensure high reliability and maintainability. All features and bug fixes must include comprehensive tests covering success and failure scenarios.

---
