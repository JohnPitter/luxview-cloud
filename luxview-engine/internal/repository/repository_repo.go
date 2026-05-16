package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type RepositoryRepo struct {
	db *DB
}

func NewRepositoryRepo(db *DB) *RepositoryRepo {
	return &RepositoryRepo{db: db}
}

func (r *RepositoryRepo) Create(ctx context.Context, repo *model.Repository) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO repositories (id, user_id, name, slug, default_branch, storage_path, visibility)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING created_at, updated_at`,
		repo.ID, repo.UserID, repo.Name, repo.Slug, repo.DefaultBranch, repo.StoragePath, repo.Visibility,
	).Scan(&repo.CreatedAt, &repo.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create repository: %w", err)
	}
	return nil
}

func (r *RepositoryRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Repository, error) {
	var repo model.Repository
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, slug, default_branch, storage_path, visibility, created_at, updated_at
		 FROM repositories WHERE id = $1`, id,
	).Scan(&repo.ID, &repo.UserID, &repo.Name, &repo.Slug, &repo.DefaultBranch, &repo.StoragePath, &repo.Visibility, &repo.CreatedAt, &repo.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find repository by id: %w", err)
	}
	return &repo, nil
}

func (r *RepositoryRepo) FindByUserAndSlug(ctx context.Context, userID uuid.UUID, slug string) (*model.Repository, error) {
	var repo model.Repository
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, slug, default_branch, storage_path, visibility, created_at, updated_at
		 FROM repositories WHERE user_id = $1 AND slug = $2`, userID, slug,
	).Scan(&repo.ID, &repo.UserID, &repo.Name, &repo.Slug, &repo.DefaultBranch, &repo.StoragePath, &repo.Visibility, &repo.CreatedAt, &repo.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find repository by user and slug: %w", err)
	}
	return &repo, nil
}

func (r *RepositoryRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Repository, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM repositories WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count repositories: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, slug, default_branch, storage_path, visibility, created_at, updated_at
		 FROM repositories WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	var repos []model.Repository
	for rows.Next() {
		var repo model.Repository
		if err := rows.Scan(&repo.ID, &repo.UserID, &repo.Name, &repo.Slug, &repo.DefaultBranch, &repo.StoragePath, &repo.Visibility, &repo.CreatedAt, &repo.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan repository: %w", err)
		}
		repos = append(repos, repo)
	}
	return repos, total, nil
}

func (r *RepositoryRepo) CreateRemote(ctx context.Context, remote *model.RepositoryRemote) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO repository_remotes (repository_id, provider, remote_url, mode)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		remote.RepositoryID, remote.Provider, remote.RemoteURL, remote.Mode,
	).Scan(&remote.ID, &remote.CreatedAt)
	if err != nil {
		return fmt.Errorf("create repository remote: %w", err)
	}
	return nil
}

func (r *RepositoryRepo) ListRemotes(ctx context.Context, repositoryID uuid.UUID) ([]model.RepositoryRemote, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, repository_id, provider, remote_url, mode, last_sync_at, last_sync_status, last_sync_error, created_at
		 FROM repository_remotes WHERE repository_id = $1
		 ORDER BY created_at DESC`, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list repository remotes: %w", err)
	}
	defer rows.Close()

	var remotes []model.RepositoryRemote
	for rows.Next() {
		var remote model.RepositoryRemote
		if err := rows.Scan(&remote.ID, &remote.RepositoryID, &remote.Provider, &remote.RemoteURL, &remote.Mode, &remote.LastSyncAt, &remote.LastSyncStatus, &remote.LastSyncError, &remote.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repository remote: %w", err)
		}
		remotes = append(remotes, remote)
	}
	return remotes, nil
}

func (r *RepositoryRepo) UpdateRemoteSyncStatus(ctx context.Context, remoteID uuid.UUID, status model.RepositorySyncStatus, errMsg string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE repository_remotes
		 SET last_sync_at = NOW(), last_sync_status = $2, last_sync_error = $3
		 WHERE id = $1`,
		remoteID, status, errMsg)
	if err != nil {
		return fmt.Errorf("update repository remote sync status: %w", err)
	}
	return nil
}
