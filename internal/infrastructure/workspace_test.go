package infrastructure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ccheney/bd-claim/internal/domain"
)

func TestWorkspaceDiscoveryAdapter_FindWorkspaceRoot(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .beads directory
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	adapter := NewWorkspaceDiscoveryAdapter()

	// Test from subdirectory
	root, err := adapter.FindWorkspaceRoot(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmpDir {
		t.Errorf("expected root '%s', got '%s'", tmpDir, root)
	}

	// Test from workspace root
	root, err = adapter.FindWorkspaceRoot(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmpDir {
		t.Errorf("expected root '%s', got '%s'", tmpDir, root)
	}
}

func TestWorkspaceDiscoveryAdapter_FindWorkspaceRoot_NotFound(t *testing.T) {
	// Create a temporary directory without .beads
	tmpDir, err := os.MkdirTemp("", "workspace-test-nofound")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	adapter := NewWorkspaceDiscoveryAdapter()

	_, err = adapter.FindWorkspaceRoot(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing .beads")
	}

	claimErr, ok := err.(*domain.ClaimFailed)
	if !ok {
		t.Fatalf("expected ClaimFailed error, got %T", err)
	}
	if claimErr.ErrorCode != domain.ErrCodeWorkspaceNotFound {
		t.Errorf("expected error code WORKSPACE_NOT_FOUND, got %s", claimErr.ErrorCode)
	}
}

func TestWorkspaceDiscoveryAdapter_FindBeadsDbPath(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "db-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .beads directory with beads.db
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	dbFile := filepath.Join(beadsDir, "beads.db")
	if err := os.WriteFile(dbFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	adapter := NewWorkspaceDiscoveryAdapter()

	dbPath, err := adapter.FindBeadsDbPath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbPath != dbFile {
		t.Errorf("expected db path '%s', got '%s'", dbFile, dbPath)
	}
}

func TestWorkspaceDiscoveryAdapter_FindBeadsDbPath_NotFound(t *testing.T) {
	// Create a temporary directory without beads.db
	tmpDir, err := os.MkdirTemp("", "db-test-notfound")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .beads directory without beads.db
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	adapter := NewWorkspaceDiscoveryAdapter()

	_, err = adapter.FindBeadsDbPath(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing beads.db")
	}

	claimErr, ok := err.(*domain.ClaimFailed)
	if !ok {
		t.Fatalf("expected ClaimFailed error, got %T", err)
	}
	if claimErr.ErrorCode != domain.ErrCodeDBNotFound {
		t.Errorf("expected error code DB_NOT_FOUND, got %s", claimErr.ErrorCode)
	}
}

func TestWorkspaceDiscoveryAdapter_FindBeadsDbPath_AccessError(t *testing.T) {
	// Skip on non-Unix systems
	if os.Getenv("CI") != "" {
		t.Skip("Skipping permission test in CI")
	}

	// Create a temporary directory with .beads dir
	tmpDir, err := os.MkdirTemp("", "db-test-access")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .beads directory
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create beads.db file
	dbFile := filepath.Join(beadsDir, "beads.db")
	if err := os.WriteFile(dbFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Make the .beads directory unreadable
	if err := os.Chmod(beadsDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(beadsDir, 0755)

	adapter := NewWorkspaceDiscoveryAdapter()

	_, err = adapter.FindBeadsDbPath(tmpDir)
	if err == nil {
		t.Fatal("expected error for access denied")
	}

	claimErr, ok := err.(*domain.ClaimFailed)
	if !ok {
		t.Fatalf("expected ClaimFailed error, got %T", err)
	}
	if claimErr.ErrorCode != domain.ErrCodeUnexpected {
		t.Errorf("expected error code UNEXPECTED, got %s", claimErr.ErrorCode)
	}
}
