package application

import "github.com/ccheney/bd-claim/internal/domain"

// ClaimIssueRequest represents a request to claim an issue.
type ClaimIssueRequest struct {
	Agent     domain.AgentName
	Filters   domain.ClaimFilters
	DryRun    bool
	TimeoutMs int
}

// ClaimIssueResult represents the result of a claim attempt.
type ClaimIssueResult struct {
	Status  string         `json:"status"`
	Agent   string         `json:"agent"`
	Issue   *IssueDTO      `json:"issue"`
	Filters *FiltersDTO    `json:"filters,omitempty"`
	Error   *ClaimErrorDTO `json:"error,omitempty"`
}

// IssueDTO is a data transfer object for issue data.
type IssueDTO struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Assignee  *string  `json:"assignee"`
	Priority  int      `json:"priority"`
	Labels    []string `json:"labels"`
	IssueType string   `json:"issue_type,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// FiltersDTO is a data transfer object for claim filters.
type FiltersDTO struct {
	OnlyUnassigned bool     `json:"only_unassigned"`
	IncludeLabels  []string `json:"include_labels"`
	ExcludeLabels  []string `json:"exclude_labels"`
	MinPriority    *int     `json:"min_priority,omitempty"`
}

// ClaimErrorDTO is a data transfer object for claim errors.
type ClaimErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// IssueToDTO converts a domain Issue to an IssueDTO.
func IssueToDTO(issue *domain.Issue) *IssueDTO {
	if issue == nil {
		return nil
	}

	var assignee *string
	if issue.Assignee != nil {
		a := issue.Assignee.String()
		assignee = &a
	}

	labels := make([]string, len(issue.Labels))
	copy(labels, issue.Labels)

	return &IssueDTO{
		ID:        issue.ID.String(),
		Title:     issue.Title,
		Status:    string(issue.Status),
		Assignee:  assignee,
		Priority:  int(issue.Priority),
		Labels:    labels,
		IssueType: issue.IssueType,
		CreatedAt: issue.CreatedAt.Format("2006-01-02T15:04:05.999999-07:00"),
		UpdatedAt: issue.UpdatedAt.Format("2006-01-02T15:04:05.999999-07:00"),
	}
}

// FiltersToDTO converts domain ClaimFilters to FiltersDTO.
func FiltersToDTO(filters domain.ClaimFilters) *FiltersDTO {
	var minPriority *int
	if filters.MinPriority != nil {
		p := int(*filters.MinPriority)
		minPriority = &p
	}

	includeLabels := filters.IncludeLabels
	if includeLabels == nil {
		includeLabels = []string{}
	}

	excludeLabels := filters.ExcludeLabels
	if excludeLabels == nil {
		excludeLabels = []string{}
	}

	return &FiltersDTO{
		OnlyUnassigned: filters.OnlyUnassigned,
		IncludeLabels:  includeLabels,
		ExcludeLabels:  excludeLabels,
		MinPriority:    minPriority,
	}
}
