package domain

// IssueClaimed is emitted when an issue is successfully claimed.
type IssueClaimed struct {
	IssueID   IssueId
	Agent     AgentName
	ClaimedAt Timestamp
}

// NoIssueAvailable is emitted when no eligible issue is found.
type NoIssueAvailable struct {
	Agent     AgentName
	Filters   ClaimFilters
	CheckedAt Timestamp
}

// ClaimErrorCode represents the type of claim error.
type ClaimErrorCode string

const (
	ErrCodeDBNotFound          ClaimErrorCode = "DB_NOT_FOUND"
	ErrCodeSchemaIncompatible  ClaimErrorCode = "SCHEMA_INCOMPATIBLE"
	ErrCodeSQLiteBusy          ClaimErrorCode = "SQLITE_BUSY"
	ErrCodeWorkspaceNotFound   ClaimErrorCode = "WORKSPACE_NOT_FOUND"
	ErrCodeInvalidArgument     ClaimErrorCode = "INVALID_ARGUMENT"
	ErrCodeUnexpected          ClaimErrorCode = "UNEXPECTED"
)

// ClaimFailed is emitted when a claim attempt fails due to technical reasons.
type ClaimFailed struct {
	Agent      *AgentName
	ErrorCode  ClaimErrorCode
	Message    string
	OccurredAt Timestamp
}

// Error implements the error interface for ClaimFailed.
func (e *ClaimFailed) Error() string {
	return string(e.ErrorCode) + ": " + e.Message
}
