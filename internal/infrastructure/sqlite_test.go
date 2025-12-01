package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ccheney/bd-claim/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "sqlite-test")
	if err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	// Create schema
	schema := `
		CREATE TABLE issues (
			id TEXT PRIMARY KEY,
			title TEXT,
			description TEXT,
			status TEXT DEFAULT 'open',
			assignee TEXT,
			priority INTEGER DEFAULT 0,
			issue_type TEXT,
			created_at TEXT,
			updated_at TEXT
		);
		CREATE TABLE labels (
			issue_id TEXT,
			label TEXT,
			PRIMARY KEY (issue_id, label)
		);
		CREATE TABLE blocked_issues_cache (
			issue_id TEXT PRIMARY KEY
		);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	db.Close()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return dbPath, cleanup
}

func insertTestIssue(t *testing.T, dbPath string, id, title, status string, priority int, assignee *string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339Nano)
	_, err = db.Exec(`
		INSERT INTO issues (id, title, status, priority, assignee, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, title, status, priority, assignee, now, now)
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestLabel(t *testing.T, dbPath string, issueID, label string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO labels (issue_id, label) VALUES (?, ?)`, issueID, label)
	if err != nil {
		t.Fatal(err)
	}
}

func blockIssue(t *testing.T, dbPath string, issueID string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO blocked_issues_cache (issue_id) VALUES (?)`, issueID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewSQLiteIssueRepository(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer repo.Close()
}

func TestNewSQLiteIssueRepository_DefaultTimeout(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer repo.Close()
}

func TestNewSQLiteIssueRepository_InvalidPath(t *testing.T) {
	_, err := NewSQLiteIssueRepository("/nonexistent/path/to/db.db", 1000)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}

	claimErr, ok := err.(*domain.ClaimFailed)
	if !ok {
		t.Fatalf("expected ClaimFailed error, got %T", err)
	}
	if claimErr.ErrorCode != domain.ErrCodeDBNotFound {
		t.Errorf("expected error code DB_NOT_FOUND, got %s", claimErr.ErrorCode)
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Test Issue 1", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.NewClaimFilters()

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-1" {
		t.Errorf("expected issue ID 'issue-1', got '%s'", issue.ID)
	}
	if issue.Status != domain.StatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", issue.Status)
	}
	if issue.Assignee == nil || *issue.Assignee != agent {
		t.Error("expected assignee to be set")
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_NoIssues(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.NewClaimFilters()

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue != nil {
		t.Error("expected no issue to be claimed")
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_BlockedIssue(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Blocked Issue", "open", 1, nil)
	blockIssue(t, dbPath, "issue-1")

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.NewClaimFilters()

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue != nil {
		t.Error("expected no issue to be claimed (blocked)")
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_InProgressIssue(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "In Progress Issue", "in_progress", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.NewClaimFilters()

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue != nil {
		t.Error("expected no issue to be claimed (in_progress)")
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_WithFilters(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	assignee := "other-agent"
	insertTestIssue(t, dbPath, "issue-1", "Assigned Issue", "open", 1, &assignee)
	insertTestIssue(t, dbPath, "issue-2", "Unassigned Issue", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.ClaimFilters{OnlyUnassigned: true}

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue ID 'issue-2', got '%s'", issue.ID)
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_WithLabels(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Backend Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-1", "backend")
	insertTestIssue(t, dbPath, "issue-2", "Frontend Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-2", "frontend")

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.ClaimFilters{IncludeLabels: []string{"backend"}}

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-1" {
		t.Errorf("expected issue ID 'issue-1', got '%s'", issue.ID)
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_WithExcludeLabels(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Wontfix Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-1", "wontfix")
	insertTestIssue(t, dbPath, "issue-2", "Normal Issue", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.ClaimFilters{ExcludeLabels: []string{"wontfix"}}

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue ID 'issue-2', got '%s'", issue.ID)
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_WithMinPriority(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Low Priority", "open", 0, nil)
	insertTestIssue(t, dbPath, "issue-2", "High Priority", "open", 2, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	minPriority := domain.PriorityHigh
	filters := domain.ClaimFilters{MinPriority: &minPriority}

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue ID 'issue-2', got '%s'", issue.ID)
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_PriorityOrder(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Low Priority", "open", 0, nil)
	insertTestIssue(t, dbPath, "issue-2", "High Priority", "open", 2, nil)
	insertTestIssue(t, dbPath, "issue-3", "Medium Priority", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	agent, _ := domain.NewAgentName("test-agent")
	filters := domain.NewClaimFilters()

	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected highest priority issue 'issue-2', got '%s'", issue.ID)
	}
}

func TestSQLiteIssueRepository_FindOneReadyIssue(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Test Issue 1", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	filters := domain.NewClaimFilters()

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be found")
	}
	if issue.ID != "issue-1" {
		t.Errorf("expected issue ID 'issue-1', got '%s'", issue.ID)
	}
	// Status should still be open (not claimed)
	if issue.Status != domain.StatusOpen {
		t.Errorf("expected status 'open', got '%s'", issue.Status)
	}
}

func TestSQLiteIssueRepository_FindOneReadyIssue_NoIssues(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	filters := domain.NewClaimFilters()

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue != nil {
		t.Error("expected no issue to be found")
	}
}

func TestSQLiteIssueRepository_FindOneReadyIssue_WithLabels(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Backend Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-1", "backend")
	insertTestLabel(t, dbPath, "issue-1", "api")

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	filters := domain.ClaimFilters{IncludeLabels: []string{"backend"}}

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be found")
	}
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}
}

func TestIsBusyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"busy error", &domain.ClaimFailed{Message: "database is locked"}, true},
		{"sqlite busy", &domain.ClaimFailed{Message: "SQLITE_BUSY"}, true},
		{"other error", &domain.ClaimFailed{Message: "some other error"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBusyError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSQLiteIssueRepository_ClaimOneReadyIssue_Concurrency(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert multiple issues
	for i := 0; i < 5; i++ {
		insertTestIssue(t, dbPath, fmt.Sprintf("issue-%d", i), fmt.Sprintf("Issue %d", i), "open", i, nil)
	}

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Claim issues sequentially to verify no double claims
	claimed := make(map[string]bool)
	for i := 0; i < 5; i++ {
		agent, _ := domain.NewAgentName(fmt.Sprintf("agent-%d", i))
		issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, domain.NewClaimFilters())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue != nil {
			if claimed[issue.ID.String()] {
				t.Errorf("issue %s was claimed twice", issue.ID)
			}
			claimed[issue.ID.String()] = true
		}
	}

	// All 5 issues should have been claimed
	if len(claimed) != 5 {
		t.Errorf("expected 5 claimed issues, got %d", len(claimed))
	}
}

func TestSQLiteIssueRepository_FindOneReadyIssue_Filters(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	assignee := "other-agent"
	insertTestIssue(t, dbPath, "issue-1", "Assigned Issue", "open", 1, &assignee)
	insertTestIssue(t, dbPath, "issue-2", "Low Priority", "open", 0, nil)
	insertTestIssue(t, dbPath, "issue-3", "High Priority", "open", 2, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Test OnlyUnassigned filter
	filters := domain.ClaimFilters{OnlyUnassigned: true}
	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
	if issue.ID == "issue-1" {
		t.Error("should not find assigned issue")
	}

	// Test MinPriority filter
	minPriority := domain.PriorityHigh
	filters = domain.ClaimFilters{MinPriority: &minPriority}
	issue, err = repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
	if issue.ID != "issue-3" {
		t.Errorf("expected issue-3 (high priority), got %s", issue.ID)
	}
}

func TestSQLiteIssueRepository_Close(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Close should not return an error
	if err := repo.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestSQLiteIssueRepository_TimestampParsing(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert issue with different timestamp formats
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Insert with RFC3339Nano format
	_, err = db.Exec(`
		INSERT INTO issues (id, title, status, priority, created_at, updated_at)
		VALUES ('test-1', 'Issue 1', 'open', 1, '2025-01-01T12:00:00.123456789Z', '2025-01-01T12:00:00.123456789Z')
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert with alternate format
	_, err = db.Exec(`
		INSERT INTO issues (id, title, status, priority, created_at, updated_at)
		VALUES ('test-2', 'Issue 2', 'open', 1, '2025-01-02T12:00:00.123456-06:00', '2025-01-02T12:00:00.123456-06:00')
	`)
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Find issues - this tests the timestamp parsing code paths
	filters := domain.NewClaimFilters()
	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
}

func TestSQLiteIssueRepository_AllFilters(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Issue 1", "open", 2, nil)
	insertTestLabel(t, dbPath, "issue-1", "backend")
	insertTestLabel(t, dbPath, "issue-1", "api")

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// Test with all filter types combined
	minPriority := domain.PriorityMedium
	filters := domain.ClaimFilters{
		OnlyUnassigned: true,
		IncludeLabels:  []string{"backend", "api"},
		ExcludeLabels:  []string{"wontfix"},
		MinPriority:    &minPriority,
	}

	agent, _ := domain.NewAgentName("test-agent")
	issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if issue.ID != "issue-1" {
		t.Errorf("expected issue-1, got %s", issue.ID)
	}
}

func TestSQLiteIssueRepository_FindWithExcludeLabels(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Wontfix Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-1", "wontfix")
	insertTestIssue(t, dbPath, "issue-2", "Normal Issue", "open", 1, nil)
	insertTestLabel(t, dbPath, "issue-2", "backend")

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	filters := domain.ClaimFilters{
		ExcludeLabels: []string{"wontfix"},
	}

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue-2, got %s", issue.ID)
	}
}

func TestSQLiteIssueRepository_FindWithMinPriority(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestIssue(t, dbPath, "issue-1", "Low Priority", "open", 0, nil)
	insertTestIssue(t, dbPath, "issue-2", "High Priority", "open", 2, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	minPriority := domain.PriorityHigh
	filters := domain.ClaimFilters{
		MinPriority: &minPriority,
	}

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue-2, got %s", issue.ID)
	}
}

func TestSQLiteIssueRepository_FindWithOnlyUnassigned(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	assignee := "other-agent"
	insertTestIssue(t, dbPath, "issue-1", "Assigned Issue", "open", 1, &assignee)
	insertTestIssue(t, dbPath, "issue-2", "Unassigned Issue", "open", 1, nil)

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	filters := domain.ClaimFilters{
		OnlyUnassigned: true,
	}

	issue, err := repo.FindOneReadyIssue(context.Background(), filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue == nil {
		t.Fatal("expected to find an issue")
	}
	if issue.ID != "issue-2" {
		t.Errorf("expected issue-2, got %s", issue.ID)
	}
}
