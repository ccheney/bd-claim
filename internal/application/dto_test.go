package application

import (
	"testing"
	"time"

	"github.com/ccheney/bd-claim/internal/domain"
)

func TestIssueToDTO_Nil(t *testing.T) {
	dto := IssueToDTO(nil)
	if dto != nil {
		t.Error("expected nil for nil issue")
	}
}

func TestIssueToDTO(t *testing.T) {
	agent := domain.AgentName("test-agent")
	now := time.Now()
	issue := &domain.Issue{
		ID:          domain.IssueId("test-123"),
		Title:       "Test Issue",
		Description: "Test description",
		Status:      domain.StatusInProgress,
		Assignee:    &agent,
		Priority:    domain.PriorityHigh,
		Labels:      domain.LabelSet{"backend", "api"},
		IssueType:   "task",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	dto := IssueToDTO(issue)

	if dto.ID != "test-123" {
		t.Errorf("expected ID 'test-123', got '%s'", dto.ID)
	}
	if dto.Title != "Test Issue" {
		t.Errorf("expected Title 'Test Issue', got '%s'", dto.Title)
	}
	if dto.Status != "in_progress" {
		t.Errorf("expected Status 'in_progress', got '%s'", dto.Status)
	}
	if dto.Assignee == nil || *dto.Assignee != "test-agent" {
		t.Error("expected Assignee to be 'test-agent'")
	}
	if dto.Priority != 2 {
		t.Errorf("expected Priority 2, got %d", dto.Priority)
	}
	if len(dto.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(dto.Labels))
	}
	if dto.IssueType != "task" {
		t.Errorf("expected IssueType 'task', got '%s'", dto.IssueType)
	}
}

func TestIssueToDTO_NoAssignee(t *testing.T) {
	issue := &domain.Issue{
		ID:       domain.IssueId("test-123"),
		Assignee: nil,
	}

	dto := IssueToDTO(issue)
	if dto.Assignee != nil {
		t.Error("expected Assignee to be nil")
	}
}

func TestFiltersToDTO(t *testing.T) {
	minPriority := domain.PriorityHigh
	filters := domain.ClaimFilters{
		OnlyUnassigned: true,
		IncludeLabels:  []string{"backend"},
		ExcludeLabels:  []string{"wontfix"},
		MinPriority:    &minPriority,
	}

	dto := FiltersToDTO(filters)

	if !dto.OnlyUnassigned {
		t.Error("expected OnlyUnassigned to be true")
	}
	if len(dto.IncludeLabels) != 1 || dto.IncludeLabels[0] != "backend" {
		t.Error("expected IncludeLabels to contain 'backend'")
	}
	if len(dto.ExcludeLabels) != 1 || dto.ExcludeLabels[0] != "wontfix" {
		t.Error("expected ExcludeLabels to contain 'wontfix'")
	}
	if dto.MinPriority == nil || *dto.MinPriority != 2 {
		t.Error("expected MinPriority to be 2")
	}
}

func TestFiltersToDTO_Defaults(t *testing.T) {
	filters := domain.ClaimFilters{
		OnlyUnassigned: false,
		IncludeLabels:  nil,
		ExcludeLabels:  nil,
		MinPriority:    nil,
	}

	dto := FiltersToDTO(filters)

	if dto.OnlyUnassigned {
		t.Error("expected OnlyUnassigned to be false")
	}
	if dto.IncludeLabels == nil {
		t.Error("expected IncludeLabels to be empty slice, not nil")
	}
	if dto.ExcludeLabels == nil {
		t.Error("expected ExcludeLabels to be empty slice, not nil")
	}
	if dto.MinPriority != nil {
		t.Error("expected MinPriority to be nil")
	}
}
