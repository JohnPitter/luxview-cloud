package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type AlertRepo struct {
	db *DB
}

func NewAlertRepo(db *DB) *AlertRepo {
	return &AlertRepo{db: db}
}

func (r *AlertRepo) Create(ctx context.Context, a *model.Alert) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO alerts (app_id, metric, condition, threshold, channel, channel_config, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		a.AppID, a.Metric, a.Condition, a.Threshold, a.Channel, a.ChannelConfig, a.Enabled,
	).Scan(&a.ID)
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	return nil
}

func (r *AlertRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Alert, error) {
	var a model.Alert
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, metric, condition, threshold, channel, channel_config, enabled, last_triggered
		 FROM alerts WHERE id = $1`, id,
	).Scan(&a.ID, &a.AppID, &a.Metric, &a.Condition, &a.Threshold,
		&a.Channel, &a.ChannelConfig, &a.Enabled, &a.LastTriggered)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find alert: %w", err)
	}
	return &a, nil
}

func (r *AlertRepo) ListByAppID(ctx context.Context, appID uuid.UUID) ([]model.Alert, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, metric, condition, threshold, channel, channel_config, enabled, last_triggered
		 FROM alerts WHERE app_id = $1 ORDER BY metric`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.AppID, &a.Metric, &a.Condition, &a.Threshold,
			&a.Channel, &a.ChannelConfig, &a.Enabled, &a.LastTriggered); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepo) ListAllEnabled(ctx context.Context) ([]model.Alert, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, metric, condition, threshold, channel, channel_config, enabled, last_triggered
		 FROM alerts WHERE enabled = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.AppID, &a.Metric, &a.Condition, &a.Threshold,
			&a.Channel, &a.ChannelConfig, &a.Enabled, &a.LastTriggered); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepo) Update(ctx context.Context, a *model.Alert) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE alerts SET metric=$2, condition=$3, threshold=$4, channel=$5,
		 channel_config=$6, enabled=$7 WHERE id=$1`,
		a.ID, a.Metric, a.Condition, a.Threshold, a.Channel, a.ChannelConfig, a.Enabled)
	return err
}

func (r *AlertRepo) UpdateLastTriggered(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE alerts SET last_triggered=NOW() WHERE id=$1`, id)
	return err
}

func (r *AlertRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM alerts WHERE id = $1`, id)
	return err
}
