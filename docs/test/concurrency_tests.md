# Concurrency Tests

**Status:** Passed
**Context:** `bd-claim` Atomic Issue Claiming

## 1. Objective

Verify that `bd-claim` maintains **strict atomicity** (Goals 1 & 4, FR-3) when multiple agents attempt to claim tasks simultaneously. This ensures that no two agents are ever assigned the same issue, even under high contention, and that the database remains consistent (no "lock storms" or corruption).

## 2. Test Scenarios

### 2.1 Scenario A: The "Highlander" Race (Single Issue)
*There can be only one.*

*   **Pre-conditions:**
    *   **Environment:** `bd --version`, `bd --help`, and `bd quickstart` checked.
    *   A single "Ready" issue exists in the DB (Status: `open`, No blockers).
    *   5 concurrent agent processes are spawned.
*   **Action:**
    *   All 5 agents execute `bd-claim` simultaneously.
*   **Expected Result:**
    *   **Exactly 1** agent receives the issue (JSON output matches the issue).
    *   **Exactly 4** agents receive `null` (JSON: `{"issue": null, ...}`).
    *   **DB State:** The issue is assigned to the "winner" and status is `in_progress`.

### 2.2 Scenario B: The "Hungry Hungry Hippos" Race (Pool of Issues)
*Rapid consumption of a shared queue.*

*   **Pre-conditions:**
    *   10 "Ready" issues exist (IDs: `TASK-1` to `TASK-10`).
    *   4 concurrent agent processes are spawned.
*   **Action:**
    *   Each agent runs `bd-claim` in a tight loop until it receives `null`.
*   **Expected Result:**
    *   All 10 issues are claimed.
    *   **Zero overlaps:** `SELECT count(*) FROM issues GROUP BY id HAVING count(assignee) > 1` (conceptually) is 0. In reality, we check that no two agents think they own the same task.
    *   **Total Claim Count:** The sum of claims reported by agents equals 10.
    *   **Distribution:** Issues are distributed among agents (exact distribution doesn't matter, just that they are all claimed uniquely).

### 2.3 Scenario C: Lock Contention & Stability (Stress Test)

*   **Pre-conditions:**
    *   Database has typical load (100+ issues).
    *   High concurrency: 20 parallel processes.
*   **Action:**
    *   Hammer `bd-claim` repeatedly.
*   **Expected Result:**
    *   **No SQLite `database is locked` crashes.** The application should handle retries or busy timeouts gracefully (or fail with a clean error code, not a panic/corruption).
    *   **Integrity:** `beads` CLI can still read the DB during/after the test.

## 3. Implementation & Harness

### 3.1 Shell Harness (Prototype)
A simple shell script using `&` background jobs to simulate concurrency.

```bash
# Pseudocode for Scenario A
reset_db_with_1_issue
for i in {1..5}; do
  ./bd-claim --agent "agent-$i" --json > "out-$i.json" &
done
wait
# Verify only one out-*.json has an issue
```

### 3.2 Go Test Harness (Robust)
For the final test suite, we will write a Go test using `goroutines` to invoke the internal `Claim` function (or the compiled binary) to strictly control timing and assertions.

## 4. Success Criteria

*   **Double-Claim Rate:** 0%. A single double-claim is a critical failure.
*   **Data Integrity:** Database passes `PRAGMA integrity_check` after tests.
*   **Error Handling:** 100% of lock errors are handled gracefully (either retried or reported as a structured error), not crashing the process.
