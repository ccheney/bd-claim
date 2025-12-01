package domain

import (
	"testing"
	"time"
)

func TestIssue_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		issue    Issue
		expected bool
	}{
		{
			name:     "open and not blocked",
			issue:    Issue{Status: StatusOpen, Blocked: false},
			expected: true,
		},
		{
			name:     "open but blocked",
			issue:    Issue{Status: StatusOpen, Blocked: true},
			expected: false,
		},
		{
			name:     "in progress",
			issue:    Issue{Status: StatusInProgress, Blocked: false},
			expected: false,
		},
		{
			name:     "closed",
			issue:    Issue{Status: StatusClosed, Blocked: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.issue.IsReady() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.issue.IsReady())
			}
		})
	}
}

func TestIssue_CanBeClaimed(t *testing.T) {
	agent := AgentName("test-agent")
	highPriority := PriorityHigh

	tests := []struct {
		name     string
		issue    Issue
		filters  ClaimFilters
		expected bool
	}{
		{
			name:     "ready issue with no filters",
			issue:    Issue{Status: StatusOpen, Blocked: false},
			filters:  NewClaimFilters(),
			expected: true,
		},
		{
			name:     "not ready issue",
			issue:    Issue{Status: StatusInProgress, Blocked: false},
			filters:  NewClaimFilters(),
			expected: false,
		},
		{
			name:     "only unassigned - unassigned issue",
			issue:    Issue{Status: StatusOpen, Blocked: false, Assignee: nil},
			filters:  ClaimFilters{OnlyUnassigned: true},
			expected: true,
		},
		{
			name:     "only unassigned - assigned issue",
			issue:    Issue{Status: StatusOpen, Blocked: false, Assignee: &agent},
			filters:  ClaimFilters{OnlyUnassigned: true},
			expected: false,
		},
		{
			name:     "include labels - has all",
			issue:    Issue{Status: StatusOpen, Blocked: false, Labels: LabelSet{"backend", "api"}},
			filters:  ClaimFilters{IncludeLabels: []string{"backend"}},
			expected: true,
		},
		{
			name:     "include labels - missing",
			issue:    Issue{Status: StatusOpen, Blocked: false, Labels: LabelSet{"frontend"}},
			filters:  ClaimFilters{IncludeLabels: []string{"backend"}},
			expected: false,
		},
		{
			name:     "exclude labels - has excluded",
			issue:    Issue{Status: StatusOpen, Blocked: false, Labels: LabelSet{"backend", "wontfix"}},
			filters:  ClaimFilters{ExcludeLabels: []string{"wontfix"}},
			expected: false,
		},
		{
			name:     "exclude labels - no excluded",
			issue:    Issue{Status: StatusOpen, Blocked: false, Labels: LabelSet{"backend"}},
			filters:  ClaimFilters{ExcludeLabels: []string{"wontfix"}},
			expected: true,
		},
		{
			name:     "min priority - meets",
			issue:    Issue{Status: StatusOpen, Blocked: false, Priority: PriorityHigh},
			filters:  ClaimFilters{MinPriority: &highPriority},
			expected: true,
		},
		{
			name:     "min priority - below",
			issue:    Issue{Status: StatusOpen, Blocked: false, Priority: PriorityLow},
			filters:  ClaimFilters{MinPriority: &highPriority},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.issue.CanBeClaimed(tt.filters) != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.issue.CanBeClaimed(tt.filters))
			}
		})
	}
}

func TestIssue_Claim(t *testing.T) {
	issue := Issue{
		ID:        IssueId("test-123"),
		Status:    StatusOpen,
		Blocked:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	agent := AgentName("test-agent")
	now := time.Now()

	event := issue.Claim(agent, now)

	// Check issue is updated
	if issue.Status != StatusInProgress {
		t.Errorf("expected status to be in_progress, got %s", issue.Status)
	}
	if issue.Assignee == nil || *issue.Assignee != agent {
		t.Error("expected assignee to be set")
	}
	if !issue.UpdatedAt.Equal(now) {
		t.Error("expected UpdatedAt to be updated")
	}

	// Check event
	if event == nil {
		t.Fatal("expected event to be returned")
	}
	if event.IssueID != issue.ID {
		t.Errorf("expected event IssueID to be %s, got %s", issue.ID, event.IssueID)
	}
	if event.Agent != agent {
		t.Errorf("expected event Agent to be %s, got %s", agent, event.Agent)
	}
}
