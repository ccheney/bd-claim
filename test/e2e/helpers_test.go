//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupBeadsWorkspace creates a temporary beads workspace for testing.
func setupBeadsWorkspace(t *testing.T) (workDir string, cleanup func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "beads-e2e-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("Failed to init beads workspace: %v", err)
	}

	return dir, func() { os.RemoveAll(dir) }
}

// createIssue creates an issue using bd CLI and returns the issue ID.
func createIssue(t *testing.T, workDir, title string) string {
	t.Helper()

	cmd := exec.Command("bd", "create", title, "--json")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to create issue: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse create output: %v", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		t.Fatalf("No id in create response")
	}

	return id
}

// getFirstReadyIssue gets the first ready issue using bd CLI.
func getFirstReadyIssue(workDir string) string {
	cmd := exec.Command("bd", "ready", "--json")
	cmd.Dir = workDir
	output, _ := cmd.Output()

	var issues []map[string]interface{}
	if err := json.Unmarshal(output, &issues); err != nil || len(issues) == 0 {
		return ""
	}

	if id, ok := issues[0]["id"].(string); ok {
		return id
	}
	return ""
}

// updateIssue updates an issue using bd CLI.
func updateIssue(workDir, issueID, status, assignee string) error {
	cmd := exec.Command("bd", "update", issueID, "--status", status, "--assignee", assignee)
	cmd.Dir = workDir
	return cmd.Run()
}

// getBdClaimBinary finds or builds the bd-claim binary.
func getBdClaimBinary(t *testing.T) string {
	t.Helper()

	// Find project root by looking for go.mod
	wd, _ := os.Getwd()
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatal("Could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	binaryPath := filepath.Join(projectRoot, "bd-claim")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath
	}

	// Build it
	cmd := exec.Command("go", "build", "-mod=mod", "-o", binaryPath, "./cmd")
	cmd.Dir = projectRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Could not build bd-claim binary: %v", err)
	}

	return binaryPath
}

// claimWithBdClaim attempts to claim an issue using bd-claim.
func claimWithBdClaim(bdClaimPath, workDir, agentName string) (claimed bool, issueID string) {
	cmd := exec.Command(bdClaimPath, "--agent", agentName, "--json")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return false, ""
	}

	if issue, ok := result["issue"].(map[string]interface{}); ok && issue != nil {
		if id, ok := issue["id"].(string); ok {
			return true, id
		}
	}

	return false, ""
}
