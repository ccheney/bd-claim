## 10. API Design

### 10.1 Primary API: CLI Contract

**Command:** `bd-claim`

**Synopsis:**

```bash
bd-claim --agent <agent-name> [options]
```

**Required Flags:**

* `--agent <agent-name>`

  * Logical identifier of the agent/worker.
  * Used to set `assignee` in the issue.

**Optional Flags (Filters & Behavior):**

* `--label <label>` (repeatable)

  * Only consider issues containing these labels.
* `--exclude-label <label>` (repeatable)

  * Exclude issues containing these labels.
* `--min-priority <level>`

  * Minimum priority (e.g., `low`, `medium`, `high`, or numeric).
* `--only-unassigned`

  * Only consider issues where `assignee IS NULL`.
* `--workspace <path>`

  * Override auto-discovered workspace root (optional).
* `--db <path>`

  * Override auto-discovered SQLite DB path (for advanced users).
* `--dry-run`

  * Show which issue *would* be claimed without updating DB.
* `--json` (default)

  * JSON output (recommended for agents).
* `--pretty`

  * Pretty-print JSON.
* `--human`

  * Human-friendly single-line or multi-line output for humans.
* `--timeout-ms <N>`

  * Override DB busy timeout (default via config).
* `--log-level <level>`

  * Control verbosity.

**JSON Output Schema (Success):**

```json
{
  "status": "ok",
  "agent": "backend-1",
  "issue": {
    "id": "bd-7f3a",
    "status": "in_progress",
    "assignee": "backend-1",
    "priority": "high",
    "labels": ["backend", "api"],
    "title": "Implement bd-claim CLI",
    "content_hash": "sha256:...",
    "external_ref": null,
    "source_repo": ".",
    "compaction_level": 0,
    "compacted_at": null,
    "original_size": 1024,
    "created_at": "<timestamp>",
    "updated_at": "<timestamp>"
  },
  "filters": {
    "only_unassigned": true,
    "include_labels": ["backend"],
    "exclude_labels": [],
    "min_priority": "medium"
  }
}
```

**JSON Output Schema (No Issue Available):**

```json
{
  "status": "ok",
  "agent": "backend-1",
  "issue": null,
  "filters": { ...same as above... }
}
```

**JSON Output Schema (Error):**

```json
{
  "status": "error",
  "agent": "backend-1",
  "issue": null,
  "error": {
    "code": "DB_NOT_FOUND" | "SCHEMA_INCOMPATIBLE" | "SQLITE_BUSY" | "WORKSPACE_NOT_FOUND" | "INVALID_ARGUMENT" | "UNEXPECTED",
    "message": "Human-readable explanation"
  }
}
```

### 10.2 Internal Application API

**Use Case Interface:**

```ts
interface ClaimIssueRequest {
  agent: AgentName;
  filters: ClaimFilters;
  dryRun: boolean;
  timeoutMs?: number;
}

interface ClaimIssueResult {
  status: "ok" | "error";
  agent: AgentName;
  issue: Issue | null;
  filters: ClaimFilters;
  error?: ClaimError;
}
```

**Port Interfaces:**

```ts
interface IssueRepositoryPort {
  claimOneReadyIssue(
    agent: AgentName,
    filters: ClaimFilters,
    readySpec: ReadySpecification,
    timeoutMs?: number
  ): Promise<Issue | null>;
}

interface WorkspaceDiscoveryPort {
  findWorkspaceRoot(cwd: string): Promise<string>; // path to repo root
  findBeadsDbPath(workspaceRoot: string): Promise<string>; // path to beads.db
}
```
