package application

import (
	"context"
	"errors"

	"github.com/ccheney/bd-claim/internal/domain"
)

// ClaimIssueUseCase handles the issue claiming workflow.
type ClaimIssueUseCase struct {
	repo   IssueRepositoryPort
	clock  ClockPort
	logger LoggerPort
}

// NewClaimIssueUseCase creates a new ClaimIssueUseCase.
func NewClaimIssueUseCase(
	repo IssueRepositoryPort,
	clock ClockPort,
	logger LoggerPort,
) *ClaimIssueUseCase {
	return &ClaimIssueUseCase{
		repo:   repo,
		clock:  clock,
		logger: logger,
	}
}

// Execute performs the claim operation.
func (uc *ClaimIssueUseCase) Execute(ctx context.Context, req ClaimIssueRequest) ClaimIssueResult {
	uc.logger.Info("claim_attempt_started", map[string]interface{}{
		"agent":   req.Agent.String(),
		"dry_run": req.DryRun,
	})

	// Dry-run mode: just find without claiming
	if req.DryRun {
		return uc.executeDryRun(ctx, req)
	}

	// Actual claim
	issue, err := uc.repo.ClaimOneReadyIssue(ctx, req.Agent, req.Filters)
	if err != nil {
		return uc.handleError(req.Agent, req.Filters, err)
	}

	if issue == nil {
		uc.logger.Info("no_issue_available", map[string]interface{}{
			"agent": req.Agent.String(),
		})
		return ClaimIssueResult{
			Status:  "ok",
			Agent:   req.Agent.String(),
			Issue:   nil,
			Filters: FiltersToDTO(req.Filters),
		}
	}

	uc.logger.Info("issue_claimed", map[string]interface{}{
		"agent":    req.Agent.String(),
		"issue_id": issue.ID.String(),
	})

	return ClaimIssueResult{
		Status:  "ok",
		Agent:   req.Agent.String(),
		Issue:   IssueToDTO(issue),
		Filters: FiltersToDTO(req.Filters),
	}
}

func (uc *ClaimIssueUseCase) executeDryRun(ctx context.Context, req ClaimIssueRequest) ClaimIssueResult {
	issue, err := uc.repo.FindOneReadyIssue(ctx, req.Filters)
	if err != nil {
		return uc.handleError(req.Agent, req.Filters, err)
	}

	uc.logger.Info("dry_run_complete", map[string]interface{}{
		"agent":       req.Agent.String(),
		"found_issue": issue != nil,
	})

	return ClaimIssueResult{
		Status:  "ok",
		Agent:   req.Agent.String(),
		Issue:   IssueToDTO(issue),
		Filters: FiltersToDTO(req.Filters),
	}
}

func (uc *ClaimIssueUseCase) handleError(agent domain.AgentName, filters domain.ClaimFilters, err error) ClaimIssueResult {
	var claimFailed *domain.ClaimFailed
	if errors.As(err, &claimFailed) {
		uc.logger.Error("claim_failed", map[string]interface{}{
			"agent": agent.String(),
			"code":  string(claimFailed.ErrorCode),
			"error": claimFailed.Message,
		})
		return ClaimIssueResult{
			Status:  "error",
			Agent:   agent.String(),
			Issue:   nil,
			Filters: FiltersToDTO(filters),
			Error: &ClaimErrorDTO{
				Code:    string(claimFailed.ErrorCode),
				Message: claimFailed.Message,
			},
		}
	}

	uc.logger.Error("unexpected_error", map[string]interface{}{
		"agent": agent.String(),
		"error": err.Error(),
	})
	return ClaimIssueResult{
		Status:  "error",
		Agent:   agent.String(),
		Issue:   nil,
		Filters: FiltersToDTO(filters),
		Error: &ClaimErrorDTO{
			Code:    string(domain.ErrCodeUnexpected),
			Message: err.Error(),
		},
	}
}
