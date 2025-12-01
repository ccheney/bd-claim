// Package application contains use cases and port definitions.
package application

import (
	"context"

	"github.com/ccheney/bd-claim/internal/domain"
)

// IssueRepositoryPort defines the interface for issue persistence.
type IssueRepositoryPort interface {
	// ClaimOneReadyIssue atomically claims a single ready issue.
	// Returns the claimed issue or nil if no issue was available.
	ClaimOneReadyIssue(
		ctx context.Context,
		agent domain.AgentName,
		filters domain.ClaimFilters,
	) (*domain.Issue, error)

	// FindOneReadyIssue finds a ready issue without claiming it (for dry-run).
	FindOneReadyIssue(
		ctx context.Context,
		filters domain.ClaimFilters,
	) (*domain.Issue, error)
}

// WorkspaceDiscoveryPort defines the interface for discovering beads workspace.
type WorkspaceDiscoveryPort interface {
	// FindWorkspaceRoot locates the repository root containing .beads directory.
	FindWorkspaceRoot(cwd string) (string, error)

	// FindBeadsDbPath locates the beads.db file within the workspace.
	FindBeadsDbPath(workspaceRoot string) (string, error)
}

// ClockPort defines the interface for time operations.
type ClockPort interface {
	// Now returns the current time.
	Now() domain.Timestamp
}

// LoggerPort defines the interface for structured logging.
type LoggerPort interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
}
