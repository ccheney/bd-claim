package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ccheney/bd-claim/internal/domain"
)

// MockIssueRepository is a mock implementation of IssueRepositoryPort.
type MockIssueRepository struct {
	ClaimFunc func(ctx context.Context, agent domain.AgentName, filters domain.ClaimFilters) (*domain.Issue, error)
	FindFunc  func(ctx context.Context, filters domain.ClaimFilters) (*domain.Issue, error)
}

func (m *MockIssueRepository) ClaimOneReadyIssue(ctx context.Context, agent domain.AgentName, filters domain.ClaimFilters) (*domain.Issue, error) {
	if m.ClaimFunc != nil {
		return m.ClaimFunc(ctx, agent, filters)
	}
	return nil, nil
}

func (m *MockIssueRepository) FindOneReadyIssue(ctx context.Context, filters domain.ClaimFilters) (*domain.Issue, error) {
	if m.FindFunc != nil {
		return m.FindFunc(ctx, filters)
	}
	return nil, nil
}

// MockClock is a mock implementation of ClockPort.
type MockClock struct {
	now domain.Timestamp
}

func (m *MockClock) Now() domain.Timestamp {
	return m.now
}

// MockLogger is a mock implementation of LoggerPort.
type MockLogger struct {
	logs []string
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func TestClaimIssueUseCase_Execute_Success(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")
	issue := &domain.Issue{
		ID:        domain.IssueId("test-123"),
		Title:     "Test Issue",
		Status:    domain.StatusInProgress,
		Assignee:  &agent,
		Priority:  domain.PriorityHigh,
		Labels:    domain.LabelSet{"backend"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo := &MockIssueRepository{
		ClaimFunc: func(ctx context.Context, a domain.AgentName, f domain.ClaimFilters) (*domain.Issue, error) {
			return issue, nil
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  false,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue == nil {
		t.Fatal("expected issue to be returned")
	}
	if result.Issue.ID != "test-123" {
		t.Errorf("expected issue ID 'test-123', got '%s'", result.Issue.ID)
	}
}

func TestClaimIssueUseCase_Execute_NoIssue(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")

	repo := &MockIssueRepository{
		ClaimFunc: func(ctx context.Context, a domain.AgentName, f domain.ClaimFilters) (*domain.Issue, error) {
			return nil, nil
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  false,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue != nil {
		t.Error("expected issue to be nil")
	}
}

func TestClaimIssueUseCase_Execute_DryRun(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")
	issue := &domain.Issue{
		ID:        domain.IssueId("test-123"),
		Title:     "Test Issue",
		Status:    domain.StatusOpen,
		Priority:  domain.PriorityHigh,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo := &MockIssueRepository{
		FindFunc: func(ctx context.Context, f domain.ClaimFilters) (*domain.Issue, error) {
			return issue, nil
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  true,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue == nil {
		t.Fatal("expected issue to be returned")
	}
}

func TestClaimIssueUseCase_Execute_DryRunNoIssue(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")

	repo := &MockIssueRepository{
		FindFunc: func(ctx context.Context, f domain.ClaimFilters) (*domain.Issue, error) {
			return nil, nil
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  true,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", result.Status)
	}
	if result.Issue != nil {
		t.Error("expected issue to be nil")
	}
}

func TestClaimIssueUseCase_Execute_ClaimError(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")

	repo := &MockIssueRepository{
		ClaimFunc: func(ctx context.Context, a domain.AgentName, f domain.ClaimFilters) (*domain.Issue, error) {
			return nil, &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeSQLiteBusy,
				Message:    "database is busy",
				OccurredAt: domain.Now(),
			}
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  false,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be returned")
	}
	if result.Error.Code != "SQLITE_BUSY" {
		t.Errorf("expected error code 'SQLITE_BUSY', got '%s'", result.Error.Code)
	}
}

func TestClaimIssueUseCase_Execute_UnexpectedError(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")

	repo := &MockIssueRepository{
		ClaimFunc: func(ctx context.Context, a domain.AgentName, f domain.ClaimFilters) (*domain.Issue, error) {
			return nil, errors.New("unexpected error")
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  false,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be returned")
	}
	if result.Error.Code != "UNEXPECTED" {
		t.Errorf("expected error code 'UNEXPECTED', got '%s'", result.Error.Code)
	}
}

func TestClaimIssueUseCase_Execute_DryRunError(t *testing.T) {
	agent, _ := domain.NewAgentName("test-agent")

	repo := &MockIssueRepository{
		FindFunc: func(ctx context.Context, f domain.ClaimFilters) (*domain.Issue, error) {
			return nil, &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeDBNotFound,
				Message:    "database not found",
				OccurredAt: domain.Now(),
			}
		},
	}
	clock := &MockClock{now: domain.Now()}
	logger := &MockLogger{}

	useCase := NewClaimIssueUseCase(repo, clock, logger)
	req := ClaimIssueRequest{
		Agent:   agent,
		Filters: domain.NewClaimFilters(),
		DryRun:  true,
	}

	result := useCase.Execute(context.Background(), req)

	if result.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be returned")
	}
	if result.Error.Code != "DB_NOT_FOUND" {
		t.Errorf("expected error code 'DB_NOT_FOUND', got '%s'", result.Error.Code)
	}
}
