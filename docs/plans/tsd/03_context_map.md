## 3. Context Boundary & Relationships (Context Map)

### 3.1 Upstream / Downstream Contexts

1. **Beads Issue Management (Upstream Context)**

   * Owns:

     * Canonical Issue model (fields: id, status, assignee, priority, labels, blockers, etc.).
     * Issue lifecycle transitions.
     * JSONL ↔ SQLite ↔ Git synchronization.
   * Relationship:

     * **Upstream / Downstream**: Issue Claiming is downstream; it **consumes** Beads’ issue data and mutates the canonical issue records via the shared SQLite DB.
     * Integration via shared DB and domain-conforming queries.
     * **Pattern**: Anti-Corruption Layer over shared database (DB-level ACL).

2. **Beads Daemon & CLI (Sibling/Partner Context)**

   * Owns:

     * `bd` CLI commands like `bd ready`, `bd update`.
     * Daemon that manages background sync, indexing.
   * Relationship:

     * Coexisting processes operating over the same DB and workspace.
     * Issue Claiming must respect daemon locking behavior and not disrupt it.
     * No code-level coupling; integration only via DB and filesystem.

3. **Agent / Orchestrator Context (External Consumer)**

   * Owns:

     * Agent lifecycle,
     * Multi-agent orchestration, scaling, higher-level scheduling.
   * Relationship:

     * **Customers** of Issue Claiming.
     * Integration via CLI (Issue Claiming exposes **Open Host Service** via command-line interface).
     * Agents treat `bd-claim` as a black-box “get work” API.

4. **Git Repository / Workspace Infrastructure (External Infra Context)**

   * Owns:

     * Git repo,
     * `.beads` directory layout,
     * CI/CD, versioning.
   * Relationship:

     * Issue Claiming auto-discovers `.beads` workspace and DB path based on current working directory.

### 3.2 Context Map Patterns

* **Anti-Corruption Layer (ACL)** between Issue Claiming domain and Beads Issue schema:

  * Domain models (`Issue`, `IssueStatus`, `LabelSet`) wrap raw DB rows.
  * No direct schema leakage to application or agent layers.

* **Shared Database Integration** (carefully controlled):

  * Although BD is anti-pattern in large distributed systems, here it’s **explicit and local** (single machine, one SQLite file).
  * We strictly confine raw SQL to infrastructure adapters.

* **Open Host Service**:

  * CLI command `bd-claim` acts as a stable service interface for agents/orchestrators.
