package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

// TestHighlanderRace tests Scenario A: multiple agents racing for a single issue.
// "There can be only one" - exactly one agent should claim the issue.
func TestHighlanderRace(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert exactly one issue
	insertTestIssue(t, dbPath, "the-one", "The Only Issue", "open", 1, nil)

	// Create separate repo connections for each agent (simulating separate processes)
	const numAgents = 5
	var wg sync.WaitGroup
	results := make(chan *domain.Issue, numAgents)
	errors := make(chan error, numAgents)

	// Start barrier to synchronize goroutine start
	start := make(chan struct{})

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentNum int) {
			defer wg.Done()

			// Each goroutine gets its own repo connection
			repo, err := NewSQLiteIssueRepository(dbPath, 5000)
			if err != nil {
				errors <- err
				return
			}
			defer repo.Close()

			agent, _ := domain.NewAgentName(fmt.Sprintf("agent-%d", agentNum))

			// Wait for start signal
			<-start

			issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, domain.NewClaimFilters())
			if err != nil {
				errors <- err
				return
			}
			results <- issue
		}(i)
	}

	// Release all goroutines simultaneously
	close(start)
	wg.Wait()
	close(results)
	close(errors)

	// Collect errors
	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}

	// Count winners (agents that got the issue)
	winners := 0
	losers := 0
	for issue := range results {
		if issue != nil {
			winners++
			if issue.ID != "the-one" {
				t.Errorf("claimed wrong issue: %s", issue.ID)
			}
		} else {
			losers++
		}
	}

	// Exactly one winner
	if winners != 1 {
		t.Errorf("expected exactly 1 winner, got %d", winners)
	}
	if losers != numAgents-1 {
		t.Errorf("expected %d losers, got %d", numAgents-1, losers)
	}
}

// TestHungryHipposRace tests Scenario B: multiple agents consuming a pool of issues.
func TestHungryHipposRace(t *testing.T) {
	dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert 10 issues
	const numIssues = 10
	for i := 0; i < numIssues; i++ {
		insertTestIssue(t, dbPath, fmt.Sprintf("task-%d", i), fmt.Sprintf("Task %d", i), "open", 1, nil)
	}

	const numAgents = 4
	var wg sync.WaitGroup
	claimedBy := make(map[string]string) // issue ID -> agent name
	var mu sync.Mutex

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentNum int) {
			defer wg.Done()

			repo, err := NewSQLiteIssueRepository(dbPath, 5000)
			if err != nil {
				t.Errorf("failed to create repo: %v", err)
				return
			}
			defer repo.Close()

			agentName := fmt.Sprintf("hippo-%d", agentNum)
			agent, _ := domain.NewAgentName(agentName)

			// Keep claiming until no more issues
			for {
				issue, err := repo.ClaimOneReadyIssue(context.Background(), agent, domain.NewClaimFilters())
				if err != nil {
					t.Errorf("agent %s got error: %v", agentName, err)
					return
				}
				if issue == nil {
					// No more issues available
					return
				}

				mu.Lock()
				if existingAgent, exists := claimedBy[issue.ID.String()]; exists {
					t.Errorf("DOUBLE CLAIM! Issue %s claimed by both %s and %s",
						issue.ID, existingAgent, agentName)
				}
				claimedBy[issue.ID.String()] = agentName
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify all issues were claimed exactly once
	if len(claimedBy) != numIssues {
		t.Errorf("expected %d claims, got %d", numIssues, len(claimedBy))
	}

	// Verify no duplicate claims (redundant but explicit)
	issueCounts := make(map[string]int)
	for issueID := range claimedBy {
		issueCounts[issueID]++
		if issueCounts[issueID] > 1 {
			t.Errorf("issue %s claimed multiple times", issueID)
		}
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

func setupTestDBWithMetadata(t *testing.T, version string) (string, func()) {
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
		CREATE TABLE metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	if version != "" {
		_, err = db.Exec("INSERT INTO metadata (key, value) VALUES ('bd_version', ?)", version)
		if err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatal(err)
		}
	}

	db.Close()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return dbPath, cleanup
}

func TestGetBdVersion(t *testing.T) {
	dbPath, cleanup := setupTestDBWithMetadata(t, "0.27.2")
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	version, err := repo.GetBdVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.27.2" {
		t.Errorf("expected version '0.27.2', got '%s'", version)
	}
}

func TestGetBdVersion_NoMetadata(t *testing.T) {
	dbPath, cleanup := setupTestDBWithMetadata(t, "")
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	version, err := repo.GetBdVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "" {
		t.Errorf("expected empty version, got '%s'", version)
	}
}

func TestCheckVersionCompatibility_Compatible(t *testing.T) {
	dbPath, cleanup := setupTestDBWithMetadata(t, "0.27.2")
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	err = repo.CheckVersionCompatibility(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckVersionCompatibility_Incompatible(t *testing.T) {
	dbPath, cleanup := setupTestDBWithMetadata(t, "0.10.0")
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	err = repo.CheckVersionCompatibility(context.Background())
	if err == nil {
		t.Fatal("expected error for incompatible version")
	}

	claimErr, ok := err.(*domain.ClaimFailed)
	if !ok {
		t.Fatalf("expected ClaimFailed error, got %T", err)
	}
	if claimErr.ErrorCode != domain.ErrCodeSchemaIncompatible {
		t.Errorf("expected SCHEMA_INCOMPATIBLE error code, got %s", claimErr.ErrorCode)
	}
}

func TestCheckVersionCompatibility_NoVersion(t *testing.T) {
	dbPath, cleanup := setupTestDBWithMetadata(t, "")
	defer cleanup()

	repo, err := NewSQLiteIssueRepository(dbPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	// No version should be considered compatible
	err = repo.CheckVersionCompatibility(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsVersionCompatible(t *testing.T) {
	tests := []struct {
		version    string
		minVersion string
		expected   bool
	}{
		{"0.27.2", "0.20.0", true},
		{"0.20.0", "0.20.0", true},
		{"0.19.9", "0.20.0", false},
		{"1.0.0", "0.20.0", true},
		{"0.20.1", "0.20.0", true},
		{"v0.27.2", "0.20.0", true},
		{"0.10.0", "0.20.0", false},
		{"2.0.0", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s>=%s", tt.version, tt.minVersion), func(t *testing.T) {
			result := isVersionCompatible(tt.version, tt.minVersion)
			if result != tt.expected {
				t.Errorf("isVersionCompatible(%s, %s) = %v, expected %v",
					tt.version, tt.minVersion, result, tt.expected)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected [3]int
	}{
		{"0.27.2", [3]int{0, 27, 2}},
		{"1.0.0", [3]int{1, 0, 0}},
		{"v0.20.0", [3]int{0, 20, 0}},
		{"2.5", [3]int{2, 5, 0}},
		{"3", [3]int{3, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := parseVersion(tt.version)
			if result != tt.expected {
				t.Errorf("parseVersion(%s) = %v, expected %v",
					tt.version, result, tt.expected)
			}
		})
	}
}
