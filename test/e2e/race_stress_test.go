//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBdRace_Stress - 20 agents, 10 issues, 5 attempts each.
func TestBdRace_Stress(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	const numIssues = 10
	for i := 1; i <= numIssues; i++ {
		createIssue(t, workDir, fmt.Sprintf("Stress %d", i))
	}

	const numAgents, attempts = 20, 5
	var totalClaims atomic.Int32
	issueCounts := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := fmt.Sprintf("stress-%02d", n)
			for a := 0; a < attempts; a++ {
				if id := getFirstReadyIssue(workDir); id != "" {
					if updateIssue(workDir, id, "in_progress", agent) == nil {
						totalClaims.Add(1)
						mu.Lock()
						issueCounts[id]++
						mu.Unlock()
					}
				}
			}
		}(i)
	}
	wg.Wait()

	t.Logf("=== STRESS TEST (bd): %d agents x %d attempts ===", numAgents, attempts)
	t.Logf("Total claims: %d | Issues: %d", totalClaims.Load(), numIssues)

	multi := 0
	for id, count := range issueCounts {
		if count > 1 {
			multi++
			t.Logf("Issue %s claimed %d times", id, count)
		}
	}

	if totalClaims.Load() > int32(numIssues) {
		t.Logf("RACE PROVEN: %d claims for %d issues, %d multi-claimed", totalClaims.Load(), numIssues, multi)
	}
}

// TestBdClaimRace_Stress - 20 agents, 10 issues, 5 attempts each.
func TestBdClaimRace_Stress(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	const numIssues = 10
	for i := 1; i <= numIssues; i++ {
		createIssue(t, workDir, fmt.Sprintf("Stress %d", i))
	}

	bdClaim := getBdClaimBinary(t)

	const numAgents, attempts = 20, 5
	var totalClaims atomic.Int32
	claimed := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := fmt.Sprintf("stress-%02d", n)
			for a := 0; a < attempts; a++ {
				if ok, id := claimWithBdClaim(bdClaim, workDir, agent); ok {
					totalClaims.Add(1)
					mu.Lock()
					if _, exists := claimed[id]; exists {
						t.Errorf("DOUBLE CLAIM: %s", id)
					}
					claimed[id] = agent
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	t.Logf("=== STRESS TEST (bd-claim): %d agents x %d attempts ===", numAgents, attempts)
	t.Logf("Total claims: %d | Unique: %d | Issues: %d", totalClaims.Load(), len(claimed), numIssues)

	if totalClaims.Load() <= int32(numIssues) {
		t.Logf("SUCCESS: No double-claiming under stress")
	}
}

// TestBdRace_Burst - 25 agents released via barrier.
func TestBdRace_Burst(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Burst target")

	const numAgents = 25
	var wg sync.WaitGroup
	var claims atomic.Int32
	barrier := make(chan struct{})

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-barrier
			if id := getFirstReadyIssue(workDir); id != "" {
				if updateIssue(workDir, id, "in_progress", fmt.Sprintf("burst-%02d", n)) == nil {
					claims.Add(1)
				}
			}
		}(i)
	}

	time.Sleep(20 * time.Millisecond)
	close(barrier)
	wg.Wait()

	t.Logf("=== BURST (bd): %d agents | Issue: %s | Claims: %d ===", numAgents, issueID, claims.Load())

	if claims.Load() > 1 {
		t.Logf("RACE PROVEN: %d agents claimed same issue in burst", claims.Load())
	}
}

// TestBdClaimRace_Burst - 25 agents released via barrier.
func TestBdClaimRace_Burst(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	issueID := createIssue(t, workDir, "Burst target")
	bdClaim := getBdClaimBinary(t)

	const numAgents = 25
	var wg sync.WaitGroup
	var wins, nulls atomic.Int32
	barrier := make(chan struct{})

	for i := 1; i <= numAgents; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-barrier
			if ok, _ := claimWithBdClaim(bdClaim, workDir, fmt.Sprintf("burst-%02d", n)); ok {
				wins.Add(1)
			} else {
				nulls.Add(1)
			}
		}(i)
	}

	time.Sleep(20 * time.Millisecond)
	close(barrier)
	wg.Wait()

	t.Logf("=== BURST (bd-claim): %d agents | Issue: %s ===", numAgents, issueID)
	t.Logf("Wins: %d | Nulls: %d", wins.Load(), nulls.Load())

	if wins.Load() != 1 {
		t.Errorf("Expected 1 winner in burst, got %d", wins.Load())
	}
}
