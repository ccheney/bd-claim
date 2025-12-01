# Beads (`bd`) Agent Workflow & Protocol

This document outlines the mandatory workflow for agents using the `bd` (Beads) CLI tool. Strict adherence ensures data integrity, clean reconciliation, and seamless multi-agent coordination.

Run `bd quickstart` for general usage information.

## 1. Issue Management
*   **Proactive Filing:** Create issues for any discovered bugs, technical debt, or TODOs encountered during the session.
*   **Naming & Detail:** Be detailed but not too verbose. Be creative with naming tasks, always staying within known character limits.
*   **Update Status:** Close completed issues and update the status of in-progress work.
*   **Batching:** You may make multiple `bd` calls (create/update/close). The tool buffers writes (30s debounce), so a final sync is required (see Section 3).

## 2. Quality Gates
*   **Trigger:** Run ONLY if code changes were made.
*   **Actions:** Execute tests, linters, and build commands.
*   **Failure Protocol:** If the build/tests are broken and cannot be fixed immediately, file a **P0** issue before ending the session.

## 3. Synchronization & Persistence (CRITICAL)
**The session is NOT complete until changes are safely pushed to the remote.**

### A. The `bd sync` Command
Always run this command after finishing issue modifications. It performs the following atomically:
1.  Exports pending changes to JSONL (bypassing the 30s debounce).
2.  Commits changes to git.
3.  Pulls from remote.
4.  Imports any updates.
5.  Pushes to remote.

```bash
bd sync
```

### B. Manual Conflict Resolution
If `bd sync` fails due to git conflicts (specifically in `.beads/beads.jsonl`):
1.  **Pull & Rebase:** `git pull --rebase`
2.  **Accept Remote (Safe Strategy):**
    ```bash
    git checkout --theirs .beads/beads.jsonl
    bd import -i .beads/beads.jsonl
    ```
    *Or perform a thoughtful manual merge if required.*
3.  **Retry Sync:** `bd sync`

### C. Verification
Ensure the push actually succeeded.
```bash
git push  # Redundant check to ensure upstream is synced
git status
```
*   **Requirement:** Output must show "up to date with origin/main".
*   **Rule:** NEVER leave work stranded locally. Retrying until success is mandatory.

## 4. Housekeeping
Ensure a clean environment for the next agent/session.
```bash
git stash clear           # Remove old stashes
git remote prune origin   # Clean up deleted remote branches
```

## 5. Session Handover
Generate a specific prompt for the next session to maintain context.

**Output Format:**
> "Continue work on bd-X: [issue title]. [Brief context about what was completed, what remains, and any architectural notes]"

## Checklist for End-of-Session
1.  [ ] **Summary:** What was completed?
2.  [ ] **Issues:** What new items were filed?
3.  [ ] **Quality:** Status of tests/lints (Pass/Fail).
4.  [ ] **Persistence:** CONFIRMATION that `bd sync` and `git push` executed successfully.
5.  [ ] **Next Steps:** The recommended prompt for the next session.
