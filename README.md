# bd-claim

A specialized CLI tool for atomically claiming "ready" issues from a [Beads](https://github.com/steveyegge/beads) SQLite database. Designed for multi-agent local swarms to prevent race conditions and duplicate work.

## Installation

### macOS (Homebrew)

```bash
brew tap ccheney/homebrew-tap
brew install bd-claim
```

### Windows (Scoop)

```powershell
scoop bucket add ccheney https://github.com/ccheney/scoop-bucket
scoop install bd-claim
```

### Linux

Download the latest release for your architecture:

```bash
# AMD64
curl -LO https://github.com/ccheney/bd-claim/releases/latest/download/bd-claim_linux_amd64.tar.gz
tar -xzf bd-claim_linux_amd64.tar.gz
sudo mv bd-claim /usr/local/bin/

# ARM64
curl -LO https://github.com/ccheney/bd-claim/releases/latest/download/bd-claim_linux_arm64.tar.gz
tar -xzf bd-claim_linux_arm64.tar.gz
sudo mv bd-claim /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/ccheney/bd-claim.git
cd bd-claim
go build -o bd-claim ./cmd
```

---

## Multi-Agent Coordination in Action

Launch 30 agents simultaneously fighting for a single issue? No problem. bd-claim ensures exactly **one winner**:

```
=== SWARM TEST: 30 agents vs 1 issue ===
Wins: 1 | Graceful exits: 29
```

Scale up to 50 agents competing for 20 issues across multiple attempts:

```
=== MASSIVE SWARM: 50 agents x 20 issues ===
Total claims: 20 | Unique: 20 | Available: 20
```

Every agent gets a fair shot. No double-claims. No wasted work.

### Lightning Fast

Each atomic claim completes in approximately **~10ms**, making bd-claim ideal for high-frequency agent polling without introducing latency into your workflow.

---

## Prerequisites & Reference

`bd-claim` is a companion tool to the `bd` CLI. It operates on the same `.beads` workspace and database.

*   **Beads CLI (`bd`):** Ensure `bd` is installed and accessible.
*   **Documentation:** Refer to the **`bd` CLI Reference** for core concepts (issues, dependencies, sync logic).
	*   Run `bd quickstart` for command usage.
	*   Run `bd --help` for bd help.
	*   See `vendor/beads/docs/CLI_REFERENCE.md` for comprehensive documentation.
	*   See `vendor/beads/docs/` for advanced topics (Daemon, Config, etc.).

## Problem It Solves

Naïve multi-agent workflow:

1. Each agent runs `bd ready --json` to find a ready issue.
2. They all pick the “top” one.
3. They *then* mark it `status=in_progress` manually.

If you start several agents at once, they will all see the same ready issue and all start working on it. There is no atomic “claim” in that pattern.

`bd-claim` changes that:

> **One command → “give me *one* ready issue and atomically set it to `in_progress` with my assignee.”**

It guarantees that, **within a single `.beads` workspace**:

* At most **one agent** successfully claims any given issue,
* Agents that lose the race will be told “no issue claimed this time” (or given a different issue), not a false positive.

No double-claims. No “we all started the same task” weirdness.

---

## AGENTS.md snippet (how you’ll instruct your agents)

Here’s the conceptual contract you’ll give to agents (no code, just behavior):

> * To start a new task, **never** call `bd ready` directly.
> * Instead, always call:
>
>   * `bd-claim --agent <YOUR_AGENT_NAME> --json`
> * If the JSON includes a non-null `"issue"`:
>
>   * You now own that issue. Use its `"id"` (e.g. `"bd-7f3a"`) for all further work.
>   * Update its description/comments with progress using `bd` commands.
>   * When you finish, update its status (e.g. `closed`) using `bd`.
> * If `"issue"` is `null`:
>
>   * There is currently no ready work for you. You may idle, exit, or retry later.

That’s the core of integrating `bd-claim` into your local swarm.

---

## High-level behavior

* Agents call:

  ```bash
  bd-claim --agent <agent-name> --json
  ```

* `bd-claim` talks to the **same Beads database** that `bd` uses, and performs a **single, atomic DB transaction** that:

  1. Selects one “ready” issue (equivalent to what `bd ready` would show),
  2. Updates it to `status=in_progress` and `assignee=<agent-name>`,
  3. Commits if and only if no one else claimed it concurrently.

* Output:

  * If an issue was successfully claimed:

    ```json
    {
      "status": "ok",
      "agent": "backend-1",
      "issue": {
        "id": "bd-7f3a",
        "title": "Implement user signup flow",
        "status": "in_progress",
        "assignee": "backend-1",
        "priority": 1,
        "labels": ["backend", "auth"],
        "created_at": "...",
        "updated_at": "..."
      }
    }
    ```

  * If **no issue could be claimed** (no ready tasks, or all lose races):

    ```json
    {
      "status": "ok",
      "agent": "backend-1",
      "issue": null
    }
    ```

  * On hard error (no DB, Beads not initialized, etc.), `status:"error"` with an error code and message.

Agents never call `bd ready` directly to pick tasks; they always go through `bd-claim`.

---

## Quickstart (conceptual)

1. **Install Beads** and initialize your repo:

   ```bash
   bd init
   bd daemon
   ```

2. **Ensure you have issues**:

   ```bash
   bd create "Implement feature X" --json
   bd create "Fix bug Y" --json
   # etc.
   ```

4. **Run multiple agents** on the same machine — each will call `bd-claim` and get a **different issue** (or `null` if none available).

---

## Local Swarm Guarantee

Within a single `.beads` workspace on one machine:

* `bd-claim` ensures **only one agent can move an issue from “ready” to “in_progress + assigned”**.
* It uses a **single SQLite write transaction** via the same storage layer that `bd` uses, so it respects Beads’ locking and sync model.

It **does not** introduce its own server; it’s just another well-behaved client of the Beads database.

---
