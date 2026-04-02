package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type AppRepo struct {
	db *DB
}

func NewAppRepo(db *DB) *AppRepo {
	return &AppRepo{db: db}
}

func (r *AppRepo) Create(ctx context.Context, app *model.App) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO apps (user_id, name, subdomain, repo_url, repo_branch, stack, status, env_vars, resource_limits, auto_deploy, custom_dockerfile, custom_domain)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, created_at, updated_at`,
		app.UserID, app.Name, app.Subdomain, app.RepoURL, app.RepoBranch,
		app.Stack, app.Status, app.EnvVars, mustJSON(app.ResourceLimits), app.AutoDeploy, app.CustomDockerfile, app.CustomDomain,
	).Scan(&app.ID, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}
	return nil
}

func (r *AppRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.App, error) {
	var app model.App
	var rl json.RawMessage
	var assignedPort *int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE id = $1`, id,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.ContainerID,
		&app.InternalPort, &assignedPort, &app.EnvVars, &rl,
		&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find app by id: %w", err)
	}
	if assignedPort != nil {
		app.AssignedPort = *assignedPort
	}
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	return &app, nil
}

func (r *AppRepo) FindBySubdomain(ctx context.Context, subdomain string) (*model.App, error) {
	var app model.App
	var rl json.RawMessage
	var assignedPort *int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE subdomain = $1`, subdomain,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.ContainerID,
		&app.InternalPort, &assignedPort, &app.EnvVars, &rl,
		&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find app by subdomain: %w", err)
	}
	if assignedPort != nil {
		app.AssignedPort = *assignedPort
	}
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	return &app, nil
}

func (r *AppRepo) FindByCustomDomain(ctx context.Context, domain string) (*model.App, error) {
	var app model.App
	var rl json.RawMessage
	var assignedPort *int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE custom_domain = $1`, domain,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.ContainerID,
		&app.InternalPort, &assignedPort, &app.EnvVars, &rl,
		&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find app by custom domain: %w", err)
	}
	if assignedPort != nil {
		app.AssignedPort = *assignedPort
	}
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	return &app, nil
}

func (r *AppRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.App, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM apps WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		var rl json.RawMessage
		var assignedPort *int
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		_ = json.Unmarshal(rl, &app.ResourceLimits)
		apps = append(apps, app)
	}
	return apps, total, nil
}

func (r *AppRepo) ListAllRunning(ctx context.Context) ([]model.App, error) {
	return r.ListAllRunningOrError(ctx)
}

func (r *AppRepo) ListAllRunningOrError(ctx context.Context) ([]model.App, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE status IN ('running', 'error', 'maintenance', 'building', 'deploying')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		var rl json.RawMessage
		var assignedPort *int
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		_ = json.Unmarshal(rl, &app.ResourceLimits)
		apps = append(apps, app)
	}
	return apps, nil
}

func (r *AppRepo) ListAll(ctx context.Context, limit, offset int) ([]model.App, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM apps`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, subdomain, repo_url, repo_branch, stack, status,
		        container_id, internal_port, assigned_port, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		var rl json.RawMessage
		var assignedPort *int
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		_ = json.Unmarshal(rl, &app.ResourceLimits)
		apps = append(apps, app)
	}
	return apps, total, nil
}

// ListAllSubdomains returns all app id+subdomain pairs for analytics subdomain resolution.
func (r *AppRepo) ListAllSubdomains(ctx context.Context) ([]model.App, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, subdomain FROM apps`)
	if err != nil {
		return nil, fmt.Errorf("list all subdomains: %w", err)
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		if err := rows.Scan(&app.ID, &app.Subdomain); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (r *AppRepo) Update(ctx context.Context, app *model.App) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE apps SET name=$2, subdomain=$3, repo_url=$4, repo_branch=$5,
		 stack=$6, status=$7, container_id=$8, internal_port=$9, assigned_port=$10,
		 env_vars=$11, resource_limits=$12, auto_deploy=$13, webhook_id=$14, custom_dockerfile=$15, custom_domain=$16, updated_at=NOW()
		 WHERE id=$1`,
		app.ID, app.Name, app.Subdomain, app.RepoURL, app.RepoBranch,
		app.Stack, app.Status, app.ContainerID, app.InternalPort, app.AssignedPort,
		app.EnvVars, mustJSON(app.ResourceLimits), app.AutoDeploy, app.WebhookID, app.CustomDockerfile, app.CustomDomain,
	)
	if err != nil {
		return fmt.Errorf("update app: %w", err)
	}
	return nil
}

func (r *AppRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus, containerID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE apps SET status=$2, container_id=$3, updated_at=NOW() WHERE id=$1`,
		id, status, containerID)
	return err
}

func (r *AppRepo) UpdatePort(ctx context.Context, id uuid.UUID, assignedPort int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE apps SET assigned_port=$2, updated_at=NOW() WHERE id=$1`,
		id, assignedPort)
	return err
}

func (r *AppRepo) UpdateCustomDockerfile(ctx context.Context, id uuid.UUID, dockerfile *string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE apps SET custom_dockerfile=$2, updated_at=NOW() WHERE id=$1`,
		id, dockerfile)
	if err != nil {
		return fmt.Errorf("update custom dockerfile: %w", err)
	}
	return nil
}

func (r *AppRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM apps WHERE id = $1`, id)
	return err
}

func (r *AppRepo) GetUsedPorts(ctx context.Context) (map[int]bool, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT assigned_port FROM apps WHERE assigned_port IS NOT NULL AND assigned_port > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ports := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		ports[p] = true
	}
	return ports, nil
}

func (r *AppRepo) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM apps`).Scan(&count)
	return count, err
}

func (r *AppRepo) CountByStatus(ctx context.Context, status model.AppStatus) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM apps WHERE status = $1`, status).Scan(&count)
	return count, err
}

func (r *AppRepo) UpdateResourceLimits(ctx context.Context, id uuid.UUID, limits model.ResourceLimits) error {
	limitsJSON, _ := json.Marshal(limits)
	_, err := r.db.Pool.Exec(ctx, `UPDATE apps SET resource_limits = $1 WHERE id = $2`, limitsJSON, id)
	if err != nil {
		return fmt.Errorf("update resource limits: %w", err)
	}
	return nil
}

func mustJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
