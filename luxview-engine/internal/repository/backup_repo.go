package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type BackupRepo struct {
	db *DB
}

func NewBackupRepo(db *DB) *BackupRepo {
	return &BackupRepo{db: db}
}

func (r *BackupRepo) Create(ctx context.Context, b *model.Backup) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO backups (databases, status, trigger, file_path, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, started_at, created_at`,
		b.Databases, b.Status, b.Trigger, b.FilePath, b.CreatedBy,
	).Scan(&b.ID, &b.StartedAt, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	return nil
}

func (r *BackupRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Backup, error) {
	var b model.Backup
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
		&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find backup by id: %w", err)
	}
	return &b, nil
}

func (r *BackupRepo) List(ctx context.Context, limit, offset int) ([]model.Backup, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM backups`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count backups: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups ORDER BY started_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		if err := rows.Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
			&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, total, nil
}

func (r *BackupRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.BackupStatus, errMsg string, fileSize int64, durationMs int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE backups SET status = $1, error = $2, file_size = $3, duration_ms = $4, completed_at = NOW()
		 WHERE id = $5`,
		status, errMsg, fileSize, durationMs, id)
	if err != nil {
		return fmt.Errorf("update backup status: %w", err)
	}
	return nil
}

func (r *BackupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	return nil
}

func (r *BackupRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM backups WHERE started_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old backups: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *BackupRepo) FindLatest(ctx context.Context) (*model.Backup, error) {
	var b model.Backup
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups ORDER BY started_at DESC LIMIT 1`,
	).Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
		&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest backup: %w", err)
	}
	return &b, nil
}
