// Package domain contains the core business logic for bd-claim.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// IssueId represents a unique identifier for an issue.
type IssueId string

// String returns the string representation of the IssueId.
func (id IssueId) String() string {
	return string(id)
}

// IsEmpty returns true if the IssueId is empty.
func (id IssueId) IsEmpty() bool {
	return id == ""
}

// AgentName represents the name of an agent claiming an issue.
type AgentName string

const maxAgentNameLength = 64

var agentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// NewAgentName creates a validated AgentName.
func NewAgentName(name string) (AgentName, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("agent name cannot be empty")
	}
	if len(name) > maxAgentNameLength {
		return "", errors.New("agent name exceeds maximum length of 64 characters")
	}
	if !agentNamePattern.MatchString(name) {
		return "", errors.New("agent name contains invalid characters; only alphanumeric, underscore, and hyphen are allowed")
	}
	return AgentName(name), nil
}

// String returns the string representation of the AgentName.
func (a AgentName) String() string {
	return string(a)
}

// IssueStatus represents the status of an issue.
type IssueStatus string

const (
	StatusOpen       IssueStatus = "open"
	StatusInProgress IssueStatus = "in_progress"
	StatusClosed     IssueStatus = "closed"
	StatusBlocked    IssueStatus = "blocked"
	StatusArchived   IssueStatus = "archived"
)

// IsClaimable returns true if the status allows claiming.
func (s IssueStatus) IsClaimable() bool {
	return s == StatusOpen
}

// Priority represents the priority level of an issue.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityMedium Priority = 1
	PriorityHigh   Priority = 2
)

// LabelSet represents a collection of labels on an issue.
type LabelSet []string

// Contains returns true if the label set contains the given label.
func (ls LabelSet) Contains(label string) bool {
	for _, l := range ls {
		if l == label {
			return true
		}
	}
	return false
}

// ContainsAny returns true if the label set contains any of the given labels.
func (ls LabelSet) ContainsAny(labels []string) bool {
	for _, label := range labels {
		if ls.Contains(label) {
			return true
		}
	}
	return false
}

// ContainsAll returns true if the label set contains all of the given labels.
func (ls LabelSet) ContainsAll(labels []string) bool {
	for _, label := range labels {
		if !ls.Contains(label) {
			return false
		}
	}
	return true
}

// ClaimFilters represents the filtering options for claiming issues.
type ClaimFilters struct {
	OnlyUnassigned bool
	IncludeLabels  []string
	ExcludeLabels  []string
	MinPriority    *Priority
}

// NewClaimFilters creates a new ClaimFilters with default values.
func NewClaimFilters() ClaimFilters {
	return ClaimFilters{
		OnlyUnassigned: false,
		IncludeLabels:  nil,
		ExcludeLabels:  nil,
		MinPriority:    nil,
	}
}

// Timestamp represents a point in time.
type Timestamp time.Time

// Now returns the current timestamp.
func Now() Timestamp {
	return Timestamp(time.Now())
}

// Time returns the underlying time.Time value.
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// String returns the RFC3339 string representation.
func (t Timestamp) String() string {
	return time.Time(t).Format(time.RFC3339)
}
