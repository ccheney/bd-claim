# Test Plan: Transitive Blocking Logic

**Status:** Final

## 1. Objective

Verify that `bd-claim` respects transitive blocking relationships. Specifically, if Issue A blocks Issue B, and Issue B is a parent of Issue C, then Issue C should be considered **blocked** and not claimable, even if Issue C itself has no direct blockers.

## 2. Test Scenario: Transitive Blocking (Parent -> Child)

### 2.1 Pre-conditions

1.  **Environment Verification:**
    *   Run `bd --version` to confirm the exact version of Beads being tested against.
	*   Run `bd quickstart` to verify usage.
    *   Run `bd --help` to verify the CLI is responsive and compatible with expected flags.
2.  **Initialize Workspace:** A fresh `.beads` workspace with SQLite DB.
3.  **Issues Setup:**
    *   **Issue A (Blocker):** Status `open`, ID `A`.
    *   **Issue B (Parent):** Status `open`, ID `B`.
    *   **Issue C (Child):** Status `open`, ID `C`.
3.  **Dependencies Setup:**
    *   `A blocks B` (Strong blocking).
    *   `B parent-of C` (Parent-child hierarchy).

### 2.2 Expected State (Before Claim)

*   **Issue A:** Ready (assuming no other blockers).
*   **Issue B:** Blocked (by A).
*   **Issue C:** Blocked (transitively via Parent B).

### 2.3 Test Steps

1.  **Run `bd-claim` for Agent 1:**
    *   Command: `bd-claim --agent agent-1 --json`
    *   **Expected Result:**
        *   Should claim **Issue A**.
        *   Output JSON should show `issue.id = "A"`.

2.  **Run `bd-claim` for Agent 2:**
    *   Command: `bd-claim --agent agent-2 --json`
    *   **Expected Result:**
        *   Should **NOT** claim Issue B (it is blocked by A, which is now `in_progress` but still conceptually blocks B until closed/resolved, depending on specific blocking semantics. Usually `open` or `in_progress` blockers prevent downstream work).
        *   *Correction based on beads logic:* Often, an item is blocked if its blocker is *not closed*. If A is `in_progress`, it is not closed, so B is still blocked.
        *   Should **NOT** claim Issue C (transitively blocked by B).
        *   Result: `issue: null` (assuming no other work exists).

3.  **Close Issue A:**
    *   Simulate work completion.
    *   Update Issue A status to `closed`.

4.  **Run `bd-claim` for Agent 2 (Retry):**
    *   Command: `bd-claim --agent agent-2 --json`
    *   **Expected Result:**
        *   Issue B is now Ready.
        *   Issue C is now Ready (parent is ready).
        *   Should claim **Issue B** (or C, depending on sort order/priority).

### 2.4 Verification Query

Ideally, we can inspect the `blocked_issues_cache` table directly to verify the internal state matches expectations.

```sql
-- Expectation after Setup:
SELECT count(*) FROM blocked_issues_cache WHERE issue_id IN ('B', 'C');
-- Should return 2.
```

## 3. Implementation Notes

*   This test requires a harness that can script `bd` or manipulate the SQLite DB directly to set up the state.
*   Since `bd-claim` is a consumer of the DB, the test setup should use `beads` libraries or CLI to create the issues and dependencies to ensure all internal caches (like `blocked_issues_cache`) are correctly updated.

## 4. Success Criteria

*   **Pass Rate:** 100% of test scenarios must pass.
*   **Code Coverage:** The implementation of the transitive blocking logic must achieve **100% code coverage**, including all error branches (e.g., DB read failures during ready check).
