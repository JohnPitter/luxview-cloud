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
	if app.AppType == "" {
		app.AppType = model.AppTypeWeb
	}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO apps (user_id, name, subdomain, repository_id, repo_url, repo_branch, stack, status, app_type, env_vars, resource_limits, auto_deploy, custom_dockerfile, custom_domain)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id, created_at, updated_at`,
		app.UserID, app.Name, app.Subdomain, app.RepositoryID, app.RepoURL, app.RepoBranch,
		app.Stack, app.Status, app.AppType, app.EnvVars, mustJSON(app.ResourceLimits), app.AutoDeploy, app.CustomDockerfile, app.CustomDomain,
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
	var repositoryID *uuid.UUID
	var gc gameConfigNullable
	err := r.db.Pool.QueryRow(ctx,
		`SELECT a.id, a.user_id, a.name, a.subdomain, a.repository_id, a.repo_url, a.repo_branch,
		        a.stack, a.status, a.app_type, a.container_id, a.internal_port, a.assigned_port,
		        a.env_vars, a.resource_limits, a.auto_deploy, a.webhook_id, a.custom_dockerfile,
		        a.custom_domain, a.created_at, a.updated_at,
		        g.id, g.template_id, g.image, g.game_port, g.query_port, g.data_dir, g.data_volume, g.volumes, g.protocol, g.config_fields
		 FROM apps a
		 LEFT JOIN game_server_configs g ON g.app_id = a.id
		 WHERE a.id = $1`, id,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &repositoryID, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.AppType, &app.ContainerID,
		&app.InternalPort, &assignedPort, &app.EnvVars, &rl,
		&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt,
		&gc.ID, &gc.TemplateID, &gc.Image, &gc.GamePort, &gc.QueryPort, &gc.DataDir, &gc.DataVolume, &gc.Volumes, &gc.Protocol, &gc.ConfigFields)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find app by id: %w", err)
	}
	if assignedPort != nil {
		app.AssignedPort = *assignedPort
	}
	app.RepositoryID = repositoryID
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	app.GameConfig = gc.toModel()
	return &app, nil
}

func (r *AppRepo) FindBySubdomain(ctx context.Context, subdomain string) (*model.App, error) {
	var app model.App
	var rl json.RawMessage
	var assignedPort *int
	var repositoryID *uuid.UUID
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, subdomain, repository_id, repo_url, repo_branch, stack, status, app_type,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE subdomain = $1`, subdomain,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &repositoryID, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.AppType, &app.ContainerID,
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
	app.RepositoryID = repositoryID
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	return &app, nil
}

func (r *AppRepo) FindByCustomDomain(ctx context.Context, domain string) (*model.App, error) {
	var app model.App
	var rl json.RawMessage
	var assignedPort *int
	var repositoryID *uuid.UUID
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, subdomain, repository_id, repo_url, repo_branch, stack, status, app_type,
		        container_id, internal_port, assigned_port, env_vars, resource_limits,
		        auto_deploy, webhook_id, custom_dockerfile, custom_domain, created_at, updated_at
		 FROM apps WHERE custom_domain = $1`, domain,
	).Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain, &repositoryID, &app.RepoURL,
		&app.RepoBranch, &app.Stack, &app.Status, &app.AppType, &app.ContainerID,
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
	app.RepositoryID = repositoryID
	_ = json.Unmarshal(rl, &app.ResourceLimits)
	return &app, nil
}

func (r *AppRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.App, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM apps WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT a.id, a.user_id, a.name, a.subdomain, a.repository_id, a.repo_url, a.repo_branch,
		        a.stack, a.status, a.app_type, a.container_id, a.internal_port, a.assigned_port,
		        a.resource_limits, a.auto_deploy, a.webhook_id, a.custom_dockerfile, a.custom_domain,
		        a.created_at, a.updated_at,
		        g.id, g.template_id, g.image, g.game_port, g.query_port, g.data_dir, g.data_volume, g.volumes, g.protocol, g.config_fields
		 FROM apps a
		 LEFT JOIN game_server_configs g ON g.app_id = a.id
		 WHERE a.user_id = $1
		 ORDER BY a.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []model.App
	for rows.Next() {
		var app model.App
		var rl json.RawMessage
		var assignedPort *int
		var repositoryID *uuid.UUID
		var gc gameConfigNullable
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&repositoryID, &app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status, &app.AppType,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain,
			&app.CreatedAt, &app.UpdatedAt,
			&gc.ID, &gc.TemplateID, &gc.Image, &gc.GamePort, &gc.QueryPort, &gc.DataDir, &gc.DataVolume, &gc.Volumes, &gc.Protocol, &gc.ConfigFields); err != nil {
			return nil, 0, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		app.RepositoryID = repositoryID
		_ = json.Unmarshal(rl, &app.ResourceLimits)
		app.GameConfig = gc.toModel()
		apps = append(apps, app)
	}
	return apps, total, nil
}

func (r *AppRepo) ListAllRunning(ctx context.Context) ([]model.App, error) {
	return r.ListAllRunningOrError(ctx)
}

func (r *AppRepo) ListAllRunningOrError(ctx context.Context) ([]model.App, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, name, subdomain, repository_id, repo_url, repo_branch, stack, status, app_type,
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
		var repositoryID *uuid.UUID
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&repositoryID, &app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status, &app.AppType,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		app.RepositoryID = repositoryID
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
		`SELECT id, user_id, name, subdomain, repository_id, repo_url, repo_branch, stack, status, app_type,
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
		var repositoryID *uuid.UUID
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.Subdomain,
			&repositoryID, &app.RepoURL, &app.RepoBranch, &app.Stack, &app.Status, &app.AppType,
			&app.ContainerID, &app.InternalPort, &assignedPort, &rl,
			&app.AutoDeploy, &app.WebhookID, &app.CustomDockerfile, &app.CustomDomain, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if assignedPort != nil {
			app.AssignedPort = *assignedPort
		}
		app.RepositoryID = repositoryID
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
		`UPDATE apps SET name=$2, subdomain=$3, repository_id=$4, repo_url=$5, repo_branch=$6,
		 stack=$7, status=$8, container_id=$9, internal_port=$10, assigned_port=$11,
		 env_vars=$12, resource_limits=$13, auto_deploy=$14, webhook_id=$15, custom_dockerfile=$16, custom_domain=$17, updated_at=NOW()
		 WHERE id=$1`,
		app.ID, app.Name, app.Subdomain, app.RepositoryID, app.RepoURL, app.RepoBranch,
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

// gameConfigNullable holds nullable columns from a LEFT JOIN on game_server_configs.
type gameConfigNullable struct {
	ID           *uuid.UUID
	TemplateID   *string
	Image        *string
	GamePort     *int
	QueryPort    *int
	DataDir      *string
	DataVolume   *string
	Volumes      *json.RawMessage
	Protocol     *string
	ConfigFields *json.RawMessage
}

func (g *gameConfigNullable) toModel() *model.GameServerConfig {
	if g.ID == nil {
		return nil
	}
	cfg := &model.GameServerConfig{
		ID:         *g.ID,
		TemplateID: strVal(g.TemplateID),
		Image:      strVal(g.Image),
		GamePort:   intVal(g.GamePort),
		QueryPort:  intVal(g.QueryPort),
		DataDir:    strVal(g.DataDir),
		DataVolume: strVal(g.DataVolume),
		Protocol:   strVal(g.Protocol),
	}
	if g.ConfigFields != nil {
		_ = json.Unmarshal(*g.ConfigFields, &cfg.ConfigFields)
	}
	if g.Volumes != nil {
		_ = json.Unmarshal(*g.Volumes, &cfg.Volumes)
	}
	return cfg
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intVal(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
