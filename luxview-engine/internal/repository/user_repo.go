package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByGitHubID(ctx context.Context, githubID int64) (*model.User, error) {
	var u model.User
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, github_id, username, email, avatar_url, github_token, role, created_at, last_login_at
		 FROM users WHERE github_id = $1`, githubID,
	).Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.GitHubToken, &u.Role, &u.CreatedAt, &u.LastLoginAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by github_id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var u model.User
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, github_id, username, email, avatar_url, github_token, role, created_at, last_login_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.GitHubToken, &u.Role, &u.CreatedAt, &u.LastLoginAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) Upsert(ctx context.Context, u *model.User) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO users (github_id, username, email, avatar_url, github_token, role, last_login_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (github_id) DO UPDATE SET
		   username = EXCLUDED.username,
		   email = EXCLUDED.email,
		   avatar_url = EXCLUDED.avatar_url,
		   github_token = EXCLUDED.github_token,
		   last_login_at = NOW()
		 RETURNING id`,
		u.GitHubID, u.Username, u.Email, u.AvatarURL, u.GitHubToken, u.Role,
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	// Re-read the ID after upsert
	return r.db.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE github_id = $1`, u.GitHubID,
	).Scan(&u.ID)
}

func (r *UserRepo) ListAll(ctx context.Context, limit, offset int) ([]model.User, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, github_id, username, email, avatar_url, role, created_at, last_login_at
		 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.LastLoginAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}
