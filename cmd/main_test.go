package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ccheney/bd-claim/internal/application"
	"github.com/ccheney/bd-claim/internal/domain"
	"github.com/ccheney/bd-claim/internal/infrastructure"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "cmd-test")
	if err != nil {
		t.Fatal(err)
	}

	// Create .beads directory
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	dbPath := filepath.Join(beadsDir, "beads.db")
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

	// Insert compatible bd_version for version check
	_, err = db.Exec("INSERT INTO metadata (key, value) VALUES ('bd_version', '0.27.2')")
	if err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	db.Close()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func insertIssue(t *testing.T, workspaceRoot string, id, title string, priority int) {
	t.Helper()

	dbPath := filepath.Join(workspaceRoot, ".beads", "beads.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO issues (id, title, status, priority, created_at, updated_at)
		VALUES (?, ?, 'open', ?, datetime('now'), datetime('now'))
	`, id, title, priority)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseFlagsFromArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		check       func(*testing.T, config)
	}{
		{
			name:        "basic agent flag",
			args:        []string{"--agent", "test-agent"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if cfg.agent != "test-agent" {
					t.Errorf("expected agent 'test-agent', got '%s'", cfg.agent)
				}
			},
		},
		{
			name:        "multiple labels",
			args:        []string{"--agent", "test", "--label", "backend", "--label", "api"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if len(cfg.labels) != 2 {
					t.Errorf("expected 2 labels, got %d", len(cfg.labels))
				}
			},
		},
		{
			name:        "all flags",
			args:        []string{"--agent", "test", "--only-unassigned", "--dry-run", "--pretty", "--human", "--timeout-ms", "5000", "--log-level", "debug", "--min-priority", "2"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if !cfg.onlyUnassigned {
					t.Error("expected onlyUnassigned to be true")
				}
				if !cfg.dryRun {
					t.Error("expected dryRun to be true")
				}
				if !cfg.pretty {
					t.Error("expected pretty to be true")
				}
				if !cfg.human {
					t.Error("expected human to be true")
				}
				if cfg.timeoutMs != 5000 {
					t.Errorf("expected timeoutMs 5000, got %d", cfg.timeoutMs)
				}
				if cfg.logLevel != "debug" {
					t.Errorf("expected logLevel 'debug', got '%s'", cfg.logLevel)
				}
				if cfg.minPriority != 2 {
					t.Errorf("expected minPriority 2, got %d", cfg.minPriority)
				}
			},
		},
		{
			name:        "version flag",
			args:        []string{"--version"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if !cfg.showVersion {
					t.Error("expected showVersion to be true")
				}
			},
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			expectError: true,
			check:       nil,
		},
		{
			name:        "workspace and db flags",
			args:        []string{"--agent", "test", "--workspace", "/tmp/test", "--db", "/tmp/test.db"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if cfg.workspace != "/tmp/test" {
					t.Errorf("expected workspace '/tmp/test', got '%s'", cfg.workspace)
				}
				if cfg.dbPath != "/tmp/test.db" {
					t.Errorf("expected dbPath '/tmp/test.db', got '%s'", cfg.dbPath)
				}
			},
		},
		{
			name:        "exclude labels",
			args:        []string{"--agent", "test", "--exclude-label", "wontfix"},
			expectError: false,
			check: func(t *testing.T, cfg config) {
				if len(cfg.excludeLabels) != 1 || cfg.excludeLabels[0] != "wontfix" {
					t.Errorf("expected exclude-label 'wontfix', got %v", cfg.excludeLabels)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseFlagsFromArgs(tt.args)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestRunApp_Version(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCode := runApp([]string{"--version"})

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if buf.String() == "" {
		t.Error("expected version output")
	}
}

func TestRunApp_InvalidFlag(t *testing.T) {
	var buf bytes.Buffer
	oldStderr := stderr
	stderr = &buf
	defer func() { stderr = oldStderr }()

	exitCode := runApp([]string{"--invalid"})

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestRunApp_MissingAgent(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCode := runApp([]string{})

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestRunApp_Success(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 1)

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCode := runApp([]string{"--agent", "test-agent", "--workspace", workspaceRoot})

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestRun_MissingAgent(t *testing.T) {
	cfg := config{
		agent: "",
	}

	result := run(cfg)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil || result.Error.Code != "INVALID_ARGUMENT" {
		t.Error("expected INVALID_ARGUMENT error")
	}
}

func TestRun_InvalidAgentName(t *testing.T) {
	cfg := config{
		agent: "invalid agent name with spaces",
	}

	result := run(cfg)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil || result.Error.Code != "INVALID_ARGUMENT" {
		t.Error("expected INVALID_ARGUMENT error")
	}
}

func TestRun_NoWorkspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "no-workspace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config{
		agent:     "test-agent",
		workspace: tmpDir,
	}

	result := run(cfg)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil || result.Error.Code != "WORKSPACE_NOT_FOUND" {
		t.Errorf("expected WORKSPACE_NOT_FOUND error, got %v", result.Error)
	}
}

func TestRun_Success(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 1)

	cfg := config{
		agent:     "test-agent",
		workspace: workspaceRoot,
		timeoutMs: 1000,
	}

	result := run(cfg)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue == nil {
		t.Fatal("expected issue to be claimed")
	}
	if result.Issue.ID != "test-123" {
		t.Errorf("expected issue ID 'test-123', got '%s'", result.Issue.ID)
	}
}

func TestRun_NoIssues(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	cfg := config{
		agent:     "test-agent",
		workspace: workspaceRoot,
		timeoutMs: 1000,
	}

	result := run(cfg)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue != nil {
		t.Error("expected no issue to be claimed")
	}
}

func TestRun_DryRun(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 1)

	cfg := config{
		agent:     "test-agent",
		workspace: workspaceRoot,
		timeoutMs: 1000,
		dryRun:    true,
	}

	result := run(cfg)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue == nil {
		t.Fatal("expected issue to be found")
	}
	// Issue should still be open (not claimed)
	if result.Issue.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", result.Issue.Status)
	}
}

func TestRun_WithDbPath(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 1)

	dbPath := filepath.Join(workspaceRoot, ".beads", "beads.db")
	cfg := config{
		agent:     "test-agent",
		dbPath:    dbPath,
		timeoutMs: 1000,
	}

	result := run(cfg)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
}

func TestRun_WithFilters(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 2)

	cfg := config{
		agent:       "test-agent",
		workspace:   workspaceRoot,
		timeoutMs:   1000,
		labels:      []string{"backend"},
		minPriority: 1,
	}

	result := run(cfg)

	// No issue because it doesn't have the required label
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
}

func TestRun_InvalidDbPath(t *testing.T) {
	cfg := config{
		agent:  "test-agent",
		dbPath: "/nonexistent/path/to/db.db",
	}

	result := run(cfg)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
}

func TestErrorResult(t *testing.T) {
	result := errorResult("test-agent", domain.ErrCodeDBNotFound, "database not found")

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Agent != "test-agent" {
		t.Errorf("expected agent 'test-agent', got '%s'", result.Agent)
	}
	if result.Issue != nil {
		t.Error("expected issue to be nil")
	}
	if result.Error == nil {
		t.Fatal("expected error to be set")
	}
	if result.Error.Code != "DB_NOT_FOUND" {
		t.Errorf("expected error code 'DB_NOT_FOUND', got '%s'", result.Error.Code)
	}
}

func TestHandleDomainError_ClaimFailed(t *testing.T) {
	err := &domain.ClaimFailed{
		ErrorCode:  domain.ErrCodeSQLiteBusy,
		Message:    "database busy",
		OccurredAt: domain.Now(),
	}

	result := handleDomainError("test-agent", err)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error.Code != "SQLITE_BUSY" {
		t.Errorf("expected error code 'SQLITE_BUSY', got '%s'", result.Error.Code)
	}
}

func TestHandleDomainError_OtherError(t *testing.T) {
	err := errors.New("some random error")

	result := handleDomainError("test-agent", err)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error.Code != "UNEXPECTED" {
		t.Errorf("expected error code 'UNEXPECTED', got '%s'", result.Error.Code)
	}
}

func TestOutputJSON(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
		Issue:  nil,
	}

	cfg := config{jsonOutput: true, pretty: false}
	exitCode := outputJSON(cfg, result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestOutputJSON_Pretty(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
		Issue:  nil,
	}

	cfg := config{jsonOutput: true, pretty: true}
	exitCode := outputJSON(cfg, result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestOutputJSON_Error(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "error",
		Agent:  "test-agent",
		Error: &application.ClaimErrorDTO{
			Code:    "TEST_ERROR",
			Message: "test error",
		},
	}

	cfg := config{jsonOutput: true}
	exitCode := outputJSON(cfg, result)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestOutputHuman_Success(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	assignee := "test-agent"
	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
		Issue: &application.IssueDTO{
			ID:       "test-123",
			Title:    "Test Issue",
			Status:   "in_progress",
			Assignee: &assignee,
			Priority: 1,
			Labels:   []string{"backend"},
		},
	}

	exitCode := outputHuman(result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestOutputHuman_NoIssue(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
		Issue:  nil,
	}

	exitCode := outputHuman(result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestOutputHuman_Error(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "error",
		Agent:  "test-agent",
		Error: &application.ClaimErrorDTO{
			Code:    "TEST_ERROR",
			Message: "test error",
		},
	}

	exitCode := outputHuman(result)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected infrastructure.LogLevel
	}{
		{"debug", infrastructure.LogLevelDebug},
		{"info", infrastructure.LogLevelInfo},
		{"warn", infrastructure.LogLevelWarn},
		{"error", infrastructure.LogLevelError},
		{"unknown", infrastructure.LogLevelError},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestOutputResult_JSON(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
	}

	cfg := config{human: false, jsonOutput: true}
	exitCode := outputResult(cfg, result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestOutputResult_Human(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
	}

	cfg := config{human: true}
	exitCode := outputResult(cfg, result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestArrayFlag(t *testing.T) {
	var af arrayFlag

	// Test String
	if af.String() != "[]" {
		t.Errorf("expected '[]', got '%s'", af.String())
	}

	// Test Set
	if err := af.Set("value1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := af.Set("value2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(af) != 2 {
		t.Errorf("expected 2 values, got %d", len(af))
	}
	if af[0] != "value1" || af[1] != "value2" {
		t.Errorf("unexpected values: %v", af)
	}
}

func TestJSONOutput_Valid(t *testing.T) {
	workspaceRoot, cleanup := setupTestDB(t)
	defer cleanup()

	insertIssue(t, workspaceRoot, "test-123", "Test Issue", 1)

	cfg := config{
		agent:      "test-agent",
		workspace:  workspaceRoot,
		timeoutMs:  1000,
		jsonOutput: true,
	}

	result := run(cfg)

	// Ensure the result can be marshalled to valid JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var parsed application.ClaimIssueResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if parsed.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", parsed.Status)
	}
}

func TestRun_GetWdError(t *testing.T) {
	// This test verifies that the error path for os.Getwd is handled
	// Since we can't easily force os.Getwd to fail, we test the explicit workspace path instead
	cfg := config{
		agent:     "test-agent",
		workspace: "/nonexistent/workspace/path",
	}

	result := run(cfg)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
}

func TestOutputHuman_NoLabels(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	assignee := "test-agent"
	result := application.ClaimIssueResult{
		Status: "ok",
		Agent:  "test-agent",
		Issue: &application.IssueDTO{
			ID:       "test-123",
			Title:    "Test Issue",
			Status:   "in_progress",
			Assignee: &assignee,
			Priority: 1,
			Labels:   []string{},
		},
	}

	exitCode := outputHuman(result)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestMain_Integration(t *testing.T) {
	// Test that main() works without panic
	exitCalled := false
	exitCode := 0
	oldOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { osExit = oldOsExit }()

	// Save os.Args and restore after test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"bd-claim", "--version"}

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	main()

	if !exitCalled {
		t.Error("expected osExit to be called")
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}
