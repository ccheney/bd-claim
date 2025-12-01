//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestBdRace_Swarm30 - 30 agents fight for 1 issue using bd.
func TestBdRace_Swarm30(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Swarm target")

	const numAgents = 30
	var wg sync.WaitGroup
	var claims atomic.Int32

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if id := getFirstReadyIssue(workDir); id != "" {
				if updateIssue(workDir, id, "in_progress", fmt.Sprintf("swarm-%02d", n)) == nil {
					claims.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	t.Logf("=== SWARM 30 (bd): Issue %s | Claims: %d ===", issueID, claims.Load())

	if claims.Load() > 1 {
		t.Logf("RACE PROVEN: %d/%d agents think they own it", claims.Load(), numAgents)
	}
}

// TestBdClaimRace_Swarm30 - 30 agents fight for 1 issue using bd-claim.
func TestBdClaimRace_Swarm30(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Swarm target")
	bdClaim := getBdClaimBinary(t)

	const numAgents = 30
	var wg sync.WaitGroup
	var wins, nulls atomic.Int32

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if ok, _ := claimWithBdClaim(bdClaim, workDir, fmt.Sprintf("swarm-%02d", n)); ok {
				wins.Add(1)
			} else {
				nulls.Add(1)
			}
		}(i)
	}
	wg.Wait()

	t.Logf("=== SWARM 30 (bd-claim): Issue %s | Wins: %d | Nulls: %d ===", issueID, wins.Load(), nulls.Load())

	if wins.Load() != 1 {
		t.Errorf("Expected 1 winner, got %d", wins.Load())
	}
}

// TestBdClaimRace_MassiveSwarm - 50 agents, 20 issues.
func TestBdClaimRace_MassiveSwarm(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	const numIssues = 20
	for i := 1; i <= numIssues; i++ {
		createIssue(t, workDir, fmt.Sprintf("Massive issue %d", i))
	}

	bdClaim := getBdClaimBinary(t)

	const numAgents = 50
	var wg sync.WaitGroup
	var totalClaims atomic.Int32
	claimed := sync.Map{}

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := fmt.Sprintf("massive-%02d", n)
			for attempt := 0; attempt < 3; attempt++ {
				if ok, id := claimWithBdClaim(bdClaim, workDir, agent); ok {
					totalClaims.Add(1)
					if _, loaded := claimed.LoadOrStore(id, agent); loaded {
						t.Errorf("DOUBLE CLAIM: %s", id)
					}
				}
			}
		}(i)
	}
	wg.Wait()

	uniqueClaims := 0
	claimed.Range(func(_, _ interface{}) bool { uniqueClaims++; return true })

	t.Logf("=== MASSIVE SWARM (bd-claim): 50 agents x 20 issues ===")
	t.Logf("Total claims: %d | Unique: %d | Available: %d", totalClaims.Load(), uniqueClaims, numIssues)

	if totalClaims.Load() > int32(numIssues) {
		t.Errorf("Double claiming: %d claims for %d issues", totalClaims.Load(), numIssues)
	}
}
