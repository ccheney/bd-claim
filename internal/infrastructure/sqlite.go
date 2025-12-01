package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ccheney/bd-claim/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultBusyTimeout = 3000 // milliseconds
	maxRetries         = 3
)

// SQLiteIssueRepository implements IssueRepositoryPort using SQLite.
type SQLiteIssueRepository struct {
	db          *sql.DB
	busyTimeout int
}

// NewSQLiteIssueRepository creates a new SQLiteIssueRepository.
func NewSQLiteIssueRepository(dbPath string, busyTimeout int) (*SQLiteIssueRepository, error) {
	if busyTimeout <= 0 {
		busyTimeout = defaultBusyTimeout
	}

	dsn := fmt.Sprintf("%s?_busy_timeout=%d&_journal_mode=WAL", dbPath, busyTimeout)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeDBNotFound,
			Message:    "failed to open database: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeDBNotFound,
			Message:    "failed to connect to database: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	return &SQLiteIssueRepository{
		db:          db,
		busyTimeout: busyTimeout,
	}, nil
}

// Close closes the database connection.
func (r *SQLiteIssueRepository) Close() error {
	return r.db.Close()
}

// ClaimOneReadyIssue atomically claims a single ready issue.
func (r *SQLiteIssueRepository) ClaimOneReadyIssue(
	ctx context.Context,
	agent domain.AgentName,
	filters domain.ClaimFilters,
) (*domain.Issue, error) {
	var issue *domain.Issue
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		issue, err = r.tryClaimIssue(ctx, agent, filters)
		if err == nil {
			return issue, nil
		}

		// Check if error is retryable (SQLITE_BUSY)
		if isBusyError(err) && attempt < maxRetries-1 {
			// Exponential backoff with jitter
			backoff := time.Duration(20*(1<<attempt)) * time.Millisecond
			time.Sleep(backoff)
			continue
		}
		break
	}

	return nil, err
}

func (r *SQLiteIssueRepository) tryClaimIssue(
	ctx context.Context,
	agent domain.AgentName,
	filters domain.ClaimFilters,
) (*domain.Issue, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeSQLiteBusy,
			Message:    "failed to begin transaction: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}
	defer tx.Rollback()

	// Build the WHERE clause based on filters
	whereClause, args := r.buildWhereClause(filters)

	// Find and update in one atomic operation using a subquery
	now := time.Now()
	query := fmt.Sprintf(`
		UPDATE issues
		SET status = 'in_progress',
			assignee = ?,
			updated_at = ?
		WHERE id = (
			SELECT i.id
			FROM issues i
			LEFT JOIN blocked_issues_cache b ON i.id = b.issue_id
			WHERE i.status = 'open'
			AND b.issue_id IS NULL
			%s
			ORDER BY i.priority DESC, i.created_at ASC, i.id ASC
			LIMIT 1
		)
		AND status = 'open'
	`, whereClause)

	// Prepend agent and timestamp to args
	allArgs := append([]interface{}{agent.String(), now.Format(time.RFC3339Nano)}, args...)

	result, err := tx.ExecContext(ctx, query, allArgs...)
	if err != nil {
		if isBusyError(err) {
			return nil, &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeSQLiteBusy,
				Message:    "database is busy: " + err.Error(),
				OccurredAt: domain.Now(),
			}
		}
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to update issue: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to get rows affected: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	if rowsAffected == 0 {
		// No issue claimed - either none available or lost race
		return nil, nil
	}

	// Fetch the claimed issue
	issue, err := r.fetchClaimedIssue(ctx, tx, agent)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeSQLiteBusy,
			Message:    "failed to commit transaction: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	return issue, nil
}

func (r *SQLiteIssueRepository) fetchClaimedIssue(
	ctx context.Context,
	tx *sql.Tx,
	agent domain.AgentName,
) (*domain.Issue, error) {
	query := `
		SELECT i.id, i.title, i.description, i.status, i.assignee, i.priority,
			   i.issue_type, i.created_at, i.updated_at
		FROM issues i
		WHERE i.assignee = ? AND i.status = 'in_progress'
		ORDER BY i.updated_at DESC
		LIMIT 1
	`

	var issue domain.Issue
	var title, description, status, assignee, issueType sql.NullString
	var priority sql.NullInt64
	var createdAt, updatedAt string

	err := tx.QueryRowContext(ctx, query, agent.String()).Scan(
		&issue.ID,
		&title,
		&description,
		&status,
		&assignee,
		&priority,
		&issueType,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to fetch claimed issue: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	issue.Title = title.String
	issue.Description = description.String
	issue.Status = domain.IssueStatus(status.String)
	if assignee.Valid {
		a := domain.AgentName(assignee.String)
		issue.Assignee = &a
	}
	issue.Priority = domain.Priority(priority.Int64)
	issue.IssueType = issueType.String

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		issue.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02T15:04:05.999999-07:00", createdAt); err == nil {
		issue.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		issue.UpdatedAt = t
	} else if t, err := time.Parse("2006-01-02T15:04:05.999999-07:00", updatedAt); err == nil {
		issue.UpdatedAt = t
	}

	// Fetch labels
	labels, err := r.fetchLabels(ctx, tx, issue.ID)
	if err != nil {
		return nil, err
	}
	issue.Labels = labels

	return &issue, nil
}

func (r *SQLiteIssueRepository) fetchLabels(
	ctx context.Context,
	tx *sql.Tx,
	issueID domain.IssueId,
) (domain.LabelSet, error) {
	query := `SELECT label FROM labels WHERE issue_id = ?`
	rows, err := tx.QueryContext(ctx, query, issueID.String())
	if err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to fetch labels: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}
	defer rows.Close()

	var labels domain.LabelSet
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeUnexpected,
				Message:    "failed to scan label: " + err.Error(),
				OccurredAt: domain.Now(),
			}
		}
		labels = append(labels, label)
	}

	return labels, nil
}

// FindOneReadyIssue finds a ready issue without claiming it.
func (r *SQLiteIssueRepository) FindOneReadyIssue(
	ctx context.Context,
	filters domain.ClaimFilters,
) (*domain.Issue, error) {
	whereClause, args := r.buildWhereClause(filters)

	query := fmt.Sprintf(`
		SELECT i.id, i.title, i.description, i.status, i.assignee, i.priority,
			   i.issue_type, i.created_at, i.updated_at
		FROM issues i
		LEFT JOIN blocked_issues_cache b ON i.id = b.issue_id
		WHERE i.status = 'open'
		AND b.issue_id IS NULL
		%s
		ORDER BY i.priority DESC, i.created_at ASC, i.id ASC
		LIMIT 1
	`, whereClause)

	var issue domain.Issue
	var title, description, status, assignee, issueType sql.NullString
	var priority sql.NullInt64
	var createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&issue.ID,
		&title,
		&description,
		&status,
		&assignee,
		&priority,
		&issueType,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to query ready issue: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}

	issue.Title = title.String
	issue.Description = description.String
	issue.Status = domain.IssueStatus(status.String)
	if assignee.Valid {
		a := domain.AgentName(assignee.String)
		issue.Assignee = &a
	}
	issue.Priority = domain.Priority(priority.Int64)
	issue.IssueType = issueType.String

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		issue.CreatedAt = t
	} else if t, err := time.Parse("2006-01-02T15:04:05.999999-07:00", createdAt); err == nil {
		issue.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		issue.UpdatedAt = t
	} else if t, err := time.Parse("2006-01-02T15:04:05.999999-07:00", updatedAt); err == nil {
		issue.UpdatedAt = t
	}

	// Fetch labels (need to use db directly, not tx)
	labels, err := r.fetchLabelsFromDb(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	issue.Labels = labels

	return &issue, nil
}

func (r *SQLiteIssueRepository) fetchLabelsFromDb(
	ctx context.Context,
	issueID domain.IssueId,
) (domain.LabelSet, error) {
	query := `SELECT label FROM labels WHERE issue_id = ?`
	rows, err := r.db.QueryContext(ctx, query, issueID.String())
	if err != nil {
		return nil, &domain.ClaimFailed{
			ErrorCode:  domain.ErrCodeUnexpected,
			Message:    "failed to fetch labels: " + err.Error(),
			OccurredAt: domain.Now(),
		}
	}
	defer rows.Close()

	var labels domain.LabelSet
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, &domain.ClaimFailed{
				ErrorCode:  domain.ErrCodeUnexpected,
				Message:    "failed to scan label: " + err.Error(),
				OccurredAt: domain.Now(),
			}
		}
		labels = append(labels, label)
	}

	return labels, nil
}

func (r *SQLiteIssueRepository) buildWhereClause(filters domain.ClaimFilters) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if filters.OnlyUnassigned {
		conditions = append(conditions, "AND i.assignee IS NULL")
	}

	if filters.MinPriority != nil {
		conditions = append(conditions, "AND i.priority >= ?")
		args = append(args, int(*filters.MinPriority))
	}

	// Include labels - issue must have ALL specified labels
	for _, label := range filters.IncludeLabels {
		conditions = append(conditions, "AND EXISTS (SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.label = ?)")
		args = append(args, label)
	}

	// Exclude labels - issue must not have ANY specified labels
	for _, label := range filters.ExcludeLabels {
		conditions = append(conditions, "AND NOT EXISTS (SELECT 1 FROM labels l WHERE l.issue_id = i.id AND l.label = ?)")
		args = append(args, label)
	}

	return strings.Join(conditions, " "), args
}

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY")
}
