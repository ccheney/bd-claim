## 16. Deployment Topology Recommendations

### 16.1 Packaging

*   Package `bd-claim` as:
    *   A standalone binary (statically linked where possible).
    *   Cross-compiled for **macOS**, **Linux**, and **Windows** (supported via `make build-all`).

*   Distribution Channels:
    *   **GitHub Releases**: Primary source for raw binary artifacts (`.tar.gz` / `.zip`).
    *   **Homebrew** (macOS & Linux): Tap formula pointing to GitHub Releases.
    *   **Scoop** (Windows): Bucket manifest pointing to GitHub Releases.

### 16.2 Version Compatibility

* `bd-claim` must be versioned with Beads core:

  * Major/minor versions aligned or compatibility matrix documented.
  * On startup, `bd-claim` may:

    * Check Beads DB schema version if available.
    * Refuse operation with clear error if incompatible.

### 16.3 Runtime Environment

* Run as a normal process:

  * No daemon mode.
  * Each invocation is short-lived, performing a single claim.

* Requirements:

  * Access to:

    * Git working tree,
    * `.beads` directory,
    * SQLite DB file.

### 16.4 Topology in Local Swarm

* Typical layout:

  * Multiple agents (processes/containers) → each periodically invoke `bd-claim`:

    * `agent-1` → `bd-claim --agent agent-1 …`
    * `agent-2` → `bd-claim --agent agent-2 …`
  * All share:

    * Same working tree,
    * Same `.beads` DB file.

* No network hops; all communication is via:

  * DB,
  * Git repo,
  * Logs.

### 16.5 Future Evolution

* This bounded context can evolve independently:

  * Future multi-machine “global claim” can be a **different bounded context** (e.g., “Distributed Scheduling”) using Issue Claiming as a local primitive.
  * Current design remains valid and stable for purely local swarms.
