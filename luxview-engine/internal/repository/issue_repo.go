package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type IssueRepo struct {
	db *DB
}

func NewIssueRepo(db *DB) *IssueRepo {
	return &IssueRepo{db: db}
}

func (r *IssueRepo) NextNumber(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	var max int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(number), 0) FROM issues WHERE repository_id = $1`, repositoryID,
	).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("next issue number: %w", err)
	}
	return max + 1, nil
}

func (r *IssueRepo) Create(ctx context.Context, i *model.Issue) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO issues (repository_id, author_id, number, title, body, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		i.RepositoryID, i.AuthorID, i.Number, i.Title, i.Body, i.Status,
	).Scan(&i.ID, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create issue: %w", err)
	}
	return nil
}

func (r *IssueRepo) FindByNumber(ctx context.Context, repositoryID uuid.UUID, number int) (*model.Issue, error) {
	var i model.Issue
	var u model.User
	err := r.db.Pool.QueryRow(ctx,
		`SELECT i.id, i.repository_id, i.author_id, i.number, i.title, i.body, i.status,
		        i.created_at, i.updated_at, i.closed_at,
		        u.id, u.username, u.avatar_url
		 FROM issues i JOIN users u ON u.id = i.author_id
		 WHERE i.repository_id = $1 AND i.number = $2`, repositoryID, number,
	).Scan(&i.ID, &i.RepositoryID, &i.AuthorID, &i.Number, &i.Title, &i.Body, &i.Status,
		&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt, &u.ID, &u.Username, &u.AvatarURL)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find issue by number: %w", err)
	}
	i.Author = &u
	return &i, nil
}

func (r *IssueRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
	var i model.Issue
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, repository_id, author_id, number, title, body, status, created_at, updated_at, closed_at
		 FROM issues WHERE id = $1`, id,
	).Scan(&i.ID, &i.RepositoryID, &i.AuthorID, &i.Number, &i.Title, &i.Body, &i.Status,
		&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find issue by id: %w", err)
	}
	return &i, nil
}

func (r *IssueRepo) List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.Issue, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM issues WHERE repository_id = $1`
	countArgs := []any{repositoryID}
	if status != "" {
		countQuery += ` AND status = $2`
		countArgs = append(countArgs, status)
	}
	if err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count issues: %w", err)
	}

	query := `SELECT i.id, i.repository_id, i.author_id, i.number, i.title, i.body, i.status,
	                 i.created_at, i.updated_at, i.closed_at,
	                 u.id, u.username, u.avatar_url
	          FROM issues i JOIN users u ON u.id = i.author_id
	          WHERE i.repository_id = $1`
	args := []any{repositoryID}
	if status != "" {
		query += ` AND i.status = $2 ORDER BY i.number DESC LIMIT $3 OFFSET $4`
		args = append(args, status, limit, offset)
	} else {
		query += ` ORDER BY i.number DESC LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var i model.Issue
		var u model.User
		if err := rows.Scan(&i.ID, &i.RepositoryID, &i.AuthorID, &i.Number, &i.Title, &i.Body, &i.Status,
			&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt, &u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, 0, fmt.Errorf("scan issue: %w", err)
		}
		i.Author = &u
		issues = append(issues, i)
	}
	return issues, total, nil
}

func (r *IssueRepo) Update(ctx context.Context, id uuid.UUID, title, body string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE issues SET title = $2, body = $3, updated_at = NOW() WHERE id = $1`, id, title, body)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return nil
}

func (r *IssueRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IssueStatus) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE issues
		 SET status = $2,
		     closed_at = CASE WHEN $2 = 'closed' THEN NOW() ELSE NULL END,
		     updated_at = NOW()
		 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("update issue status: %w", err)
	}
	return nil
}

// Labels

func (r *IssueRepo) CreateLabel(ctx context.Context, l *model.Label) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO labels (repository_id, name, color, description)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		l.RepositoryID, l.Name, l.Color, l.Description,
	).Scan(&l.ID, &l.CreatedAt)
	if err != nil {
		return fmt.Errorf("create label: %w", err)
	}
	return nil
}

func (r *IssueRepo) ListLabels(ctx context.Context, repositoryID uuid.UUID) ([]model.Label, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, repository_id, name, color, description, created_at
		 FROM labels WHERE repository_id = $1 ORDER BY name ASC`, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	var labels []model.Label
	for rows.Next() {
		var l model.Label
		if err := rows.Scan(&l.ID, &l.RepositoryID, &l.Name, &l.Color, &l.Description, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, nil
}

func (r *IssueRepo) DeleteLabel(ctx context.Context, repositoryID, labelID uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM labels WHERE id = $1 AND repository_id = $2`, labelID, repositoryID)
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("label not found")
	}
	return nil
}

// SetIssueLabels replaces the label set for an issue.
func (r *IssueRepo) SetIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set labels: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM issue_labels WHERE issue_id = $1`, issueID); err != nil {
		return fmt.Errorf("clear issue labels: %w", err)
	}
	for _, labelID := range labelIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO issue_labels (issue_id, label_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, issueID, labelID); err != nil {
			return fmt.Errorf("add issue label: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *IssueRepo) LabelsForIssue(ctx context.Context, issueID uuid.UUID) ([]model.Label, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT l.id, l.repository_id, l.name, l.color, l.description, l.created_at
		 FROM labels l JOIN issue_labels il ON il.label_id = l.id
		 WHERE il.issue_id = $1 ORDER BY l.name ASC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("labels for issue: %w", err)
	}
	defer rows.Close()

	var labels []model.Label
	for rows.Next() {
		var l model.Label
		if err := rows.Scan(&l.ID, &l.RepositoryID, &l.Name, &l.Color, &l.Description, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan issue label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, nil
}

// Comments

func (r *IssueRepo) CreateComment(ctx context.Context, c *model.IssueComment) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO issue_comments (issue_id, author_id, body)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		c.IssueID, c.AuthorID, c.Body,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create issue comment: %w", err)
	}
	return nil
}

func (r *IssueRepo) ListComments(ctx context.Context, issueID uuid.UUID) ([]model.IssueComment, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT c.id, c.issue_id, c.author_id, c.body, c.created_at, c.updated_at,
		        u.id, u.username, u.avatar_url
		 FROM issue_comments c JOIN users u ON u.id = c.author_id
		 WHERE c.issue_id = $1 ORDER BY c.created_at ASC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue comments: %w", err)
	}
	defer rows.Close()

	var comments []model.IssueComment
	for rows.Next() {
		var c model.IssueComment
		var u model.User
		if err := rows.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt,
			&u.ID, &u.Username, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan issue comment: %w", err)
		}
		c.Author = &u
		comments = append(comments, c)
	}
	return comments, nil
}

func (r *IssueRepo) DeleteComment(ctx context.Context, commentID, authorID uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM issue_comments WHERE id = $1 AND author_id = $2`, commentID, authorID)
	if err != nil {
		return fmt.Errorf("delete issue comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("comment not found or not owned by user")
	}
	return nil
}
