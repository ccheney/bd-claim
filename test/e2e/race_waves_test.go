//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBdRace_CascadingWaves - 3 waves of 10 agents each.
func TestBdRace_CascadingWaves(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	const numIssues = 3
	for i := 1; i <= numIssues; i++ {
		createIssue(t, workDir, fmt.Sprintf("Wave target %d", i))
	}

	const agentsPerWave, numWaves = 10, 3
	var totalClaims atomic.Int32
	doubleClaimed := make(map[string][]string)
	var mu sync.Mutex

	for wave := 1; wave <= numWaves; wave++ {
		var wg sync.WaitGroup
		for i := 1; i <= agentsPerWave; i++ {
			wg.Add(1)
			go func(w, n int) {
				defer wg.Done()
				agent := fmt.Sprintf("wave%d-%02d", w, n)
				if id := getFirstReadyIssue(workDir); id != "" {
					if updateIssue(workDir, id, "in_progress", agent) == nil {
						totalClaims.Add(1)
						mu.Lock()
						doubleClaimed[id] = append(doubleClaimed[id], agent)
						mu.Unlock()
					}
				}
			}(wave, i)
		}
		wg.Wait()
		time.Sleep(5 * time.Millisecond)
	}

	t.Logf("=== CASCADING WAVES (bd): %d waves x %d agents ===", numWaves, agentsPerWave)
	t.Logf("Total claims: %d | Issues: %d", totalClaims.Load(), numIssues)

	doubles := 0
	for id, agents := range doubleClaimed {
		if len(agents) > 1 {
			doubles++
			t.Logf("DOUBLE CLAIM on %s: %d agents", id, len(agents))
		}
	}

	if totalClaims.Load() > int32(numIssues) {
		t.Logf("RACE PROVEN: %d claims for %d issues", totalClaims.Load(), numIssues)
	}
}

// TestBdClaimRace_CascadingWaves - 3 waves of 10 agents each using bd-claim.
func TestBdClaimRace_CascadingWaves(t *testing.T) {
	workDir, cleanup := setupBeadsWorkspace(t)
	defer cleanup()

	const numIssues = 3
	for i := 1; i <= numIssues; i++ {
		createIssue(t, workDir, fmt.Sprintf("Wave target %d", i))
	}

	bdClaim := getBdClaimBinary(t)

	const agentsPerWave, numWaves = 10, 3
	var totalClaims atomic.Int32
	claimed := make(map[string]string)
	var mu sync.Mutex

	for wave := 1; wave <= numWaves; wave++ {
		var wg sync.WaitGroup
		for i := 1; i <= agentsPerWave; i++ {
			wg.Add(1)
			go func(w, n int) {
				defer wg.Done()
				agent := fmt.Sprintf("wave%d-%02d", w, n)
				if ok, id := claimWithBdClaim(bdClaim, workDir, agent); ok {
					totalClaims.Add(1)
					mu.Lock()
					claimed[id] = agent
					mu.Unlock()
				}
			}(wave, i)
		}
		wg.Wait()
		time.Sleep(5 * time.Millisecond)
	}

	t.Logf("=== CASCADING WAVES (bd-claim): %d waves x %d agents ===", numWaves, agentsPerWave)
	t.Logf("Total claims: %d | Unique: %d | Issues: %d", totalClaims.Load(), len(claimed), numIssues)

	if totalClaims.Load() > int32(numIssues) {
		t.Errorf("Double claiming: %d claims for %d issues", totalClaims.Load(), numIssues)
	}
}
