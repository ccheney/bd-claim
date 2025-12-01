package domain

import (
	"testing"
)

func TestIssueId_String(t *testing.T) {
	id := IssueId("test-123")
	if id.String() != "test-123" {
		t.Errorf("expected 'test-123', got '%s'", id.String())
	}
}

func TestIssueId_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		id       IssueId
		expected bool
	}{
		{"empty", IssueId(""), true},
		{"non-empty", IssueId("test"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.id.IsEmpty() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.id.IsEmpty())
			}
		})
	}
}

func TestNewAgentName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{"valid simple", "agent1", false},
		{"valid with underscore", "backend_agent", false},
		{"valid with hyphen", "agent-1", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"invalid chars", "agent@1", true},
		{"invalid space", "agent 1", true},
		{"valid trimmed", "  agent1  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAgentName(tt.input)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestAgentName_String(t *testing.T) {
	agent, _ := NewAgentName("test-agent")
	if agent.String() != "test-agent" {
		t.Errorf("expected 'test-agent', got '%s'", agent.String())
	}
}

func TestIssueStatus_IsClaimable(t *testing.T) {
	tests := []struct {
		status   IssueStatus
		expected bool
	}{
		{StatusOpen, true},
		{StatusInProgress, false},
		{StatusClosed, false},
		{StatusBlocked, false},
		{StatusArchived, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if tt.status.IsClaimable() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.status.IsClaimable())
			}
		})
	}
}

func TestLabelSet_Contains(t *testing.T) {
	ls := LabelSet{"backend", "api", "urgent"}

	if !ls.Contains("backend") {
		t.Error("expected to contain 'backend'")
	}
	if ls.Contains("frontend") {
		t.Error("did not expect to contain 'frontend'")
	}
}

func TestLabelSet_ContainsAny(t *testing.T) {
	ls := LabelSet{"backend", "api"}

	if !ls.ContainsAny([]string{"frontend", "backend"}) {
		t.Error("expected ContainsAny to return true")
	}
	if ls.ContainsAny([]string{"frontend", "mobile"}) {
		t.Error("expected ContainsAny to return false")
	}
	if ls.ContainsAny([]string{}) {
		t.Error("expected ContainsAny with empty slice to return false")
	}
}

func TestLabelSet_ContainsAll(t *testing.T) {
	ls := LabelSet{"backend", "api", "urgent"}

	if !ls.ContainsAll([]string{"backend", "api"}) {
		t.Error("expected ContainsAll to return true")
	}
	if ls.ContainsAll([]string{"backend", "frontend"}) {
		t.Error("expected ContainsAll to return false")
	}
	if !ls.ContainsAll([]string{}) {
		t.Error("expected ContainsAll with empty slice to return true")
	}
}

func TestNewClaimFilters(t *testing.T) {
	filters := NewClaimFilters()
	if filters.OnlyUnassigned {
		t.Error("expected OnlyUnassigned to be false")
	}
	if filters.IncludeLabels != nil {
		t.Error("expected IncludeLabels to be nil")
	}
	if filters.ExcludeLabels != nil {
		t.Error("expected ExcludeLabels to be nil")
	}
	if filters.MinPriority != nil {
		t.Error("expected MinPriority to be nil")
	}
}

func TestTimestamp(t *testing.T) {
	now := Now()
	time := now.Time()
	if time.IsZero() {
		t.Error("expected non-zero time")
	}

	str := now.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestPriorityConstants(t *testing.T) {
	if PriorityLow != 0 {
		t.Errorf("expected PriorityLow to be 0, got %d", PriorityLow)
	}
	if PriorityMedium != 1 {
		t.Errorf("expected PriorityMedium to be 1, got %d", PriorityMedium)
	}
	if PriorityHigh != 2 {
		t.Errorf("expected PriorityHigh to be 2, got %d", PriorityHigh)
	}
}
