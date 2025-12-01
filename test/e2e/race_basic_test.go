//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestBdRaceCondition_Basic proves the naive bd workflow has a race condition.
// EXPECTS: Multiple agents will think they claimed the same issue.
func TestBdRaceCondition_Basic(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Race test issue")

	const numAgents = 5
	var wg sync.WaitGroup
	var claimCount atomic.Int32

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := fmt.Sprintf("agent-%d", n)

			if id := getFirstReadyIssue(workDir); id != "" {
				if updateIssue(workDir, id, "in_progress", agent) == nil {
					claimCount.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	t.Logf("Issue: %s | Claims: %d/%d", issueID, claimCount.Load(), numAgents)

	if claimCount.Load() > 1 {
		t.Logf("RACE PROVEN: %d agents think they own the same issue", claimCount.Load())
	}
}

// TestBdClaimPreventsRace_Basic proves bd-claim's atomic transaction prevents races.
func TestBdClaimPreventsRace_Basic(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Atomic claim test")
	bdClaim := getBdClaimBinary(t)

	const numAgents = 5
	var wg sync.WaitGroup
	var wins, losses atomic.Int32

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if ok, _ := claimWithBdClaim(bdClaim, workDir, fmt.Sprintf("agent-%d", n)); ok {
				wins.Add(1)
			} else {
				losses.Add(1)
			}
		}(i)
	}
	wg.Wait()

	t.Logf("Issue: %s | Wins: %d | Nulls: %d", issueID, wins.Load(), losses.Load())

	if wins.Load() != 1 {
		t.Errorf("Expected 1 winner, got %d", wins.Load())
	}
	if losses.Load() != numAgents-1 {
		t.Errorf("Expected %d nulls, got %d", numAgents-1, losses.Load())
	}
}
