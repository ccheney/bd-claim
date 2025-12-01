package domain

import (
	"strings"
	"testing"
)

func TestClaimFailed_Error(t *testing.T) {
	err := &ClaimFailed{
		ErrorCode:  ErrCodeDBNotFound,
		Message:    "database not found",
		OccurredAt: Now(),
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "DB_NOT_FOUND") {
		t.Errorf("expected error to contain 'DB_NOT_FOUND', got '%s'", errStr)
	}
	if !strings.Contains(errStr, "database not found") {
		t.Errorf("expected error to contain 'database not found', got '%s'", errStr)
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []ClaimErrorCode{
		ErrCodeDBNotFound,
		ErrCodeSchemaIncompatible,
		ErrCodeSQLiteBusy,
		ErrCodeWorkspaceNotFound,
		ErrCodeInvalidArgument,
		ErrCodeUnexpected,
	}

	for _, code := range codes {
		if code == "" {
			t.Error("error code should not be empty")
		}
	}
}

func TestIssueClaimed(t *testing.T) {
	agent := AgentName("test-agent")
	event := IssueClaimed{
		IssueID:   IssueId("test-123"),
		Agent:     agent,
		ClaimedAt: Now(),
	}

	if event.IssueID.IsEmpty() {
		t.Error("expected non-empty IssueID")
	}
	if event.Agent.String() == "" {
		t.Error("expected non-empty Agent")
	}
}

func TestNoIssueAvailable(t *testing.T) {
	agent := AgentName("test-agent")
	event := NoIssueAvailable{
		Agent:     agent,
		Filters:   NewClaimFilters(),
		CheckedAt: Now(),
	}

	if event.Agent.String() == "" {
		t.Error("expected non-empty Agent")
	}
}
