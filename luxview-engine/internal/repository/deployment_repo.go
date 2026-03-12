package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type DeploymentRepo struct {
	db *DB
}

func NewDeploymentRepo(db *DB) *DeploymentRepo {
	return &DeploymentRepo{db: db}
}

func (r *DeploymentRepo) Create(ctx context.Context, d *model.Deployment) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO deployments (app_id, commit_sha, commit_message, status, image_tag, source)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		d.AppID, d.CommitSHA, d.CommitMessage, d.Status, d.ImageTag, d.Source,
	).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}
	return nil
}

func (r *DeploymentRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	var d model.Deployment
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, commit_sha, commit_message, status, build_log,
		        duration_ms, image_tag, source, created_at, finished_at
		 FROM deployments WHERE id = $1`, id,
	).Scan(&d.ID, &d.AppID, &d.CommitSHA, &d.CommitMessage, &d.Status,
		&d.BuildLog, &d.DurationMs, &d.ImageTag, &d.Source, &d.CreatedAt, &d.FinishedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find deployment: %w", err)
	}
	return &d, nil
}

func (r *DeploymentRepo) ListByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]model.Deployment, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM deployments WHERE app_id = $1`, appID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, commit_sha, commit_message, status, duration_ms, image_tag, source, created_at, finished_at
		 FROM deployments WHERE app_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, appID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var deployments []model.Deployment
	for rows.Next() {
		var d model.Deployment
		if err := rows.Scan(&d.ID, &d.AppID, &d.CommitSHA, &d.CommitMessage,
			&d.Status, &d.DurationMs, &d.ImageTag, &d.Source, &d.CreatedAt, &d.FinishedAt); err != nil {
			return nil, 0, err
		}
		deployments = append(deployments, d)
	}
	return deployments, total, nil
}

func (r *DeploymentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.DeploymentStatus, buildLog string, durationMs int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE deployments SET status=$2, build_log=$3, duration_ms=$4, finished_at=NOW() WHERE id=$1`,
		id, status, buildLog, durationMs)
	return err
}

func (r *DeploymentRepo) FindLastLive(ctx context.Context, appID uuid.UUID, excludeID uuid.UUID) (*model.Deployment, error) {
	var d model.Deployment
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, commit_sha, commit_message, status, duration_ms, image_tag, source, created_at, finished_at
		 FROM deployments WHERE app_id = $1 AND status = 'live' AND id != $2
		 ORDER BY created_at DESC LIMIT 1`, appID, excludeID,
	).Scan(&d.ID, &d.AppID, &d.CommitSHA, &d.CommitMessage, &d.Status,
		&d.DurationMs, &d.ImageTag, &d.Source, &d.CreatedAt, &d.FinishedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// FailStale marks deployments stuck in pending/building for longer than the given
// timeout as failed, and resets the corresponding app status from building to error.
// Returns the number of deployment rows affected.
func (r *DeploymentRepo) FailStale(ctx context.Context, maxAge int) (int64, error) {
	interval := fmt.Sprintf("%d seconds", maxAge)

	// Reset app status for apps that have stale deployments.
	_, _ = r.db.Pool.Exec(ctx,
		`UPDATE apps SET status = 'error', updated_at = NOW()
		 WHERE status = 'building'
		   AND id IN (
		     SELECT app_id FROM deployments
		     WHERE status IN ('pending', 'building')
		       AND created_at < NOW() - $1::interval
		   )`,
		interval)

	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE deployments
		 SET status = 'failed',
		     build_log = COALESCE(build_log, '') || E'\n[timeout] Automatically marked as failed — stuck for over ' || $1,
		     finished_at = NOW()
		 WHERE status IN ('pending', 'building')
		   AND created_at < NOW() - $1::interval`,
		interval)
	if err != nil {
		return 0, fmt.Errorf("fail stale deployments: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *DeploymentRepo) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM deployments`).Scan(&count)
	return count, err
}
