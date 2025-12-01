// Package infrastructure contains adapters for external dependencies.
package infrastructure

import (
	"os"
	"path/filepath"

	"github.com/ccheney/bd-claim/internal/domain"
)

const (
	beadsDir    = ".beads"
	beadsDbFile = "beads.db"
)

// WorkspaceDiscoveryAdapter implements workspace discovery.
type WorkspaceDiscoveryAdapter struct{}

// NewWorkspaceDiscoveryAdapter creates a new WorkspaceDiscoveryAdapter.
func NewWorkspaceDiscoveryAdapter() *WorkspaceDiscoveryAdapter {
	return &WorkspaceDiscoveryAdapter{}
}

// FindWorkspaceRoot locates the repository root containing .beads directory.
func (w *WorkspaceDiscoveryAdapter) FindWorkspaceRoot(cwd string) (string, error) {
	dir := cwd
	for {
		beadsPath := filepath.Join(dir, beadsDir)
		if info, err := os.Stat(beadsPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .beads
			return "", &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeWorkspaceNotFound,
				Message:    "no .beads directory found in any parent directory",
				OccurredAt: domain.Now(),
			}
		}
		dir = parent
	}
}

// FindBeadsDbPath locates the beads.db file within the workspace.
func (w *WorkspaceDiscoveryAdapter) FindBeadsDbPath(workspaceRoot string) (string, error) {
	dbPath := filepath.Join(workspaceRoot, beadsDir, beadsDbFile)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return "", &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeDBNotFound,
				Message:    "beads.db not found at " + dbPath,
				OccurredAt: domain.Now(),
			}
		}
		return "", &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "error accessing beads.db: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}
	return dbPath, nil
}
