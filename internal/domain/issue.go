package domain

import "time"

// Issue represents an issue aggregate in the claiming context.
type Issue struct {
	ID          IssueId
	Title       string
	Description string
	Status      IssueStatus
	Assignee    *AgentName
	Priority    Priority
	Labels      LabelSet
	IssueType   string
	Blocked     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsReady returns true if the issue is eligible for claiming.
func (i *Issue) IsReady() bool {
	return i.Status.IsClaimable() && !i.Blocked
}

// CanBeClaimed checks if the issue can be claimed given the filters.
func (i *Issue) CanBeClaimed(filters ClaimFilters) bool {
	if !i.IsReady() {
		return false
	}

	// Check assignee filter
	if filters.OnlyUnassigned && i.Assignee != nil {
		return false
	}

	// Check include labels
	if len(filters.IncludeLabels) > 0 && !i.Labels.ContainsAll(filters.IncludeLabels) {
		return false
	}

	// Check exclude labels
	if len(filters.ExcludeLabels) > 0 && i.Labels.ContainsAny(filters.ExcludeLabels) {
		return false
	}

	// Check minimum priority
	if filters.MinPriority != nil && i.Priority < *filters.MinPriority {
		return false
	}

	return true
}

// Claim transitions the issue to claimed state.
func (i *Issue) Claim(agent AgentName, now time.Time) *IssueClaimed {
	i.Status = StatusInProgress
	i.Assignee = &agent
	i.UpdatedAt = now

	return &IssueClaimed{
		IssueID:   i.ID,
		Agent:     agent,
		ClaimedAt: Timestamp(now),
	}
}
