package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type PullRequestRepo struct {
	db *DB
}

func NewPullRequestRepo(db *DB) *PullRequestRepo {
	return &PullRequestRepo{db: db}
}

func (r *PullRequestRepo) Create(ctx context.Context, pr *model.PullRequest) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO pull_requests
		 (repository_id, author_id, number, title, description, head_branch, base_branch, head_sha, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		pr.RepositoryID, pr.AuthorID, pr.Number, pr.Title, pr.Description,
		pr.HeadBranch, pr.BaseBranch, pr.HeadSHA, pr.Status,
	).Scan(&pr.ID, &pr.CreatedAt, &pr.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.PullRequest, error) {
	var pr model.PullRequest
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, repository_id, author_id, number, title, description,
		        head_branch, base_branch, head_sha, status, merge_commit,
		        created_at, updated_at, merged_at, closed_at
		 FROM pull_requests WHERE id = $1`, id,
	).Scan(&pr.ID, &pr.RepositoryID, &pr.AuthorID, &pr.Number, &pr.Title, &pr.Description,
		&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.Status, &pr.MergeCommit,
		&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pull request: %w", err)
	}
	return &pr, nil
}

func (r *PullRequestRepo) FindByNumber(ctx context.Context, repositoryID uuid.UUID, number int) (*model.PullRequest, error) {
	var pr model.PullRequest
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, repository_id, author_id, number, title, description,
		        head_branch, base_branch, head_sha, status, merge_commit,
		        created_at, updated_at, merged_at, closed_at
		 FROM pull_requests WHERE repository_id = $1 AND number = $2`,
		repositoryID, number,
	).Scan(&pr.ID, &pr.RepositoryID, &pr.AuthorID, &pr.Number, &pr.Title, &pr.Description,
		&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.Status, &pr.MergeCommit,
		&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pull request by number: %w", err)
	}
	return &pr, nil
}

func (r *PullRequestRepo) List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.PullRequest, int, error) {
	var total int
	query := `SELECT COUNT(*) FROM pull_requests WHERE repository_id = $1`
	args := []any{repositoryID}
	if status != "" {
		query += ` AND status = $2`
		args = append(args, status)
	}
	if err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pull requests: %w", err)
	}

	dataQuery := `SELECT id, repository_id, author_id, number, title, description,
		        head_branch, base_branch, head_sha, status, merge_commit,
		        created_at, updated_at, merged_at, closed_at
		 FROM pull_requests WHERE repository_id = $1`
	dataArgs := []any{repositoryID}
	if status != "" {
		dataQuery += ` AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		dataArgs = append(dataArgs, status, limit, offset)
	} else {
		dataQuery += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		dataArgs = append(dataArgs, limit, offset)
	}

	rows, err := r.db.Pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pull requests: %w", err)
	}
	defer rows.Close()

	var prs []model.PullRequest
	for rows.Next() {
		var pr model.PullRequest
		if err := rows.Scan(&pr.ID, &pr.RepositoryID, &pr.AuthorID, &pr.Number, &pr.Title, &pr.Description,
			&pr.HeadBranch, &pr.BaseBranch, &pr.HeadSHA, &pr.Status, &pr.MergeCommit,
			&pr.CreatedAt, &pr.UpdatedAt, &pr.MergedAt, &pr.ClosedAt); err != nil {
			return nil, 0, fmt.Errorf("scan pull request: %w", err)
		}
		prs = append(prs, pr)
	}
	return prs, total, nil
}

func (r *PullRequestRepo) NextNumber(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	var max int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(number), 0) FROM pull_requests WHERE repository_id = $1`, repositoryID,
	).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("next pull request number: %w", err)
	}
	return max + 1, nil
}

func (r *PullRequestRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PullRequestStatus, mergeCommit *string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE pull_requests
		 SET status = $2, merge_commit = $3,
		     merged_at = CASE WHEN $2 = 'merged' THEN NOW() ELSE merged_at END,
		     closed_at = CASE WHEN $2 IN ('merged','closed') THEN NOW() ELSE closed_at END,
		     updated_at = NOW()
		 WHERE id = $1`,
		id, status, mergeCommit)
	if err != nil {
		return fmt.Errorf("update pull request status: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) UpdateHeadSHA(ctx context.Context, id uuid.UUID, sha string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE pull_requests SET head_sha = $2, updated_at = NOW() WHERE id = $1`, id, sha)
	if err != nil {
		return fmt.Errorf("update pull request head sha: %w", err)
	}
	return nil
}

// Comments

func (r *PullRequestRepo) CreateComment(ctx context.Context, c *model.PullRequestComment) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO pull_request_comments (pull_request_id, author_id, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		c.PullRequestID, c.AuthorID, c.Body,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create pull request comment: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) ListComments(ctx context.Context, prID uuid.UUID) ([]model.PullRequestComment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT c.id, c.pull_request_id, c.author_id, c.body, c.created_at, c.updated_at,
		        u.id, u.username, u.avatar_url
		 FROM pull_request_comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.pull_request_id = $1
		 ORDER BY c.created_at ASC`, prID)
	if err != nil {
		return nil, fmt.Errorf("list pull request comments: %w", err)
	}
	defer rows.Close()

	var comments []model.PullRequestComment
	for rows.Next() {
		var c model.PullRequestComment
		var u model.User
		if err := rows.Scan(&c.ID, &c.PullRequestID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt,
			&u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan pull request comment: %w", err)
		}
		c.Author = &u
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *PullRequestRepo) DeleteComment(ctx context.Context, commentID uuid.UUID, authorID uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM pull_request_comments WHERE id = $1 AND author_id = $2`, commentID, authorID)
	if err != nil {
		return fmt.Errorf("delete pull request comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("comment not found or not owned by user")
	}
	return nil
}

// Reviews

func (r *PullRequestRepo) CreateReview(ctx context.Context, rv *model.PullRequestReview) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO pull_request_reviews (pull_request_id, reviewer_id, state, body, commit_sha)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		rv.PullRequestID, rv.ReviewerID, rv.State, rv.Body, rv.CommitSHA,
	).Scan(&rv.ID, &rv.CreatedAt)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) ListReviews(ctx context.Context, prID uuid.UUID) ([]model.PullRequestReview, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT rv.id, rv.pull_request_id, rv.reviewer_id, rv.state, rv.body, rv.commit_sha, rv.created_at,
		        u.id, u.username, u.avatar_url
		 FROM pull_request_reviews rv JOIN users u ON u.id = rv.reviewer_id
		 WHERE rv.pull_request_id = $1 ORDER BY rv.created_at ASC`, prID)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	var reviews []model.PullRequestReview
	for rows.Next() {
		var rv model.PullRequestReview
		var u model.User
		if err := rows.Scan(&rv.ID, &rv.PullRequestID, &rv.ReviewerID, &rv.State, &rv.Body, &rv.CommitSHA, &rv.CreatedAt,
			&u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		rv.Reviewer = &u
		reviews = append(reviews, rv)
	}
	return reviews, nil
}

// LatestReviewStates returns the most recent decisive review (approved/changes_requested)
// per reviewer, keyed by reviewer ID. Pure "commented" reviews are ignored.
func (r *PullRequestRepo) LatestReviewStates(ctx context.Context, prID uuid.UUID) (map[uuid.UUID]model.ReviewState, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT DISTINCT ON (reviewer_id) reviewer_id, state
		 FROM pull_request_reviews
		 WHERE pull_request_id = $1 AND state IN ('approved','changes_requested')
		 ORDER BY reviewer_id, created_at DESC`, prID)
	if err != nil {
		return nil, fmt.Errorf("latest review states: %w", err)
	}
	defer rows.Close()

	states := map[uuid.UUID]model.ReviewState{}
	for rows.Next() {
		var reviewerID uuid.UUID
		var state model.ReviewState
		if err := rows.Scan(&reviewerID, &state); err != nil {
			return nil, fmt.Errorf("scan review state: %w", err)
		}
		states[reviewerID] = state
	}
	return states, nil
}

// Inline review comments

func (r *PullRequestRepo) CreateReviewComment(ctx context.Context, c *model.ReviewComment) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO pull_request_review_comments (pull_request_id, author_id, path, line, side, body)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`,
		c.PullRequestID, c.AuthorID, c.Path, c.Line, c.Side, c.Body,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create review comment: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) ListReviewComments(ctx context.Context, prID uuid.UUID) ([]model.ReviewComment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT c.id, c.pull_request_id, c.author_id, c.path, c.line, c.side, c.body, c.resolved,
		        c.created_at, c.updated_at, u.id, u.username, u.avatar_url
		 FROM pull_request_review_comments c JOIN users u ON u.id = c.author_id
		 WHERE c.pull_request_id = $1 ORDER BY c.path ASC, c.line ASC, c.created_at ASC`, prID)
	if err != nil {
		return nil, fmt.Errorf("list review comments: %w", err)
	}
	defer rows.Close()

	var comments []model.ReviewComment
	for rows.Next() {
		var c model.ReviewComment
		var u model.User
		if err := rows.Scan(&c.ID, &c.PullRequestID, &c.AuthorID, &c.Path, &c.Line, &c.Side, &c.Body, &c.Resolved,
			&c.CreatedAt, &c.UpdatedAt, &u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan review comment: %w", err)
		}
		c.Author = &u
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *PullRequestRepo) SetReviewCommentResolved(ctx context.Context, commentID uuid.UUID, resolved bool) error {
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE pull_request_review_comments SET resolved = $2, updated_at = NOW() WHERE id = $1`, commentID, resolved)
	if err != nil {
		return fmt.Errorf("resolve review comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("review comment not found")
	}
	return nil
}

func (r *PullRequestRepo) DeleteReviewComment(ctx context.Context, commentID, authorID uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM pull_request_review_comments WHERE id = $1 AND author_id = $2`, commentID, authorID)
	if err != nil {
		return fmt.Errorf("delete review comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("comment not found or not owned by user")
	}
	return nil
}

// ListStatusChecks returns the CI runs (action_runs) tied to the repository's apps
// for a given head commit — surfaced as PR status checks.
func (r *PullRequestRepo) ListStatusChecks(ctx context.Context, repositoryID uuid.UUID, commitSHA string) ([]model.StatusCheck, error) {
	if commitSHA == "" {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT ar.workflow, ar.status, ar.id, ar.commit_sha, ar.created_at, ar.finished_at
		 FROM action_runs ar JOIN apps a ON a.id = ar.app_id
		 WHERE a.repository_id = $1 AND ar.commit_sha = $2
		 ORDER BY ar.created_at DESC`, repositoryID, commitSHA)
	if err != nil {
		return nil, fmt.Errorf("list status checks: %w", err)
	}
	defer rows.Close()

	var checks []model.StatusCheck
	for rows.Next() {
		var c model.StatusCheck
		if err := rows.Scan(&c.Name, &c.Status, &c.RunID, &c.CommitSHA, &c.CreatedAt, &c.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan status check: %w", err)
		}
		checks = append(checks, c)
	}
	return checks, nil
}
