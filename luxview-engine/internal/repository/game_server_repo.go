package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type GameServerConfigRepo struct {
	db *DB
}

func NewGameServerConfigRepo(db *DB) *GameServerConfigRepo {
	return &GameServerConfigRepo{db: db}
}

func (r *GameServerConfigRepo) GetByAppID(ctx context.Context, appID uuid.UUID) (*model.GameServerConfig, error) {
	var cfg model.GameServerConfig
	var fields json.RawMessage
	var dataVolume *string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, template_id, image, game_port, query_port, data_dir, data_volume, protocol, config_fields, created_at, updated_at
		 FROM game_server_configs WHERE app_id = $1`, appID,
	).Scan(&cfg.ID, &cfg.AppID, &cfg.TemplateID, &cfg.Image, &cfg.GamePort, &cfg.QueryPort,
		&cfg.DataDir, &dataVolume, &cfg.Protocol, &fields, &cfg.CreatedAt, &cfg.UpdatedAt)
	if dataVolume != nil {
		cfg.DataVolume = *dataVolume
	}
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game server config: %w", err)
	}
	_ = json.Unmarshal(fields, &cfg.ConfigFields)
	return &cfg, nil
}

func (r *GameServerConfigRepo) Create(ctx context.Context, cfg *model.GameServerConfig) error {
	fields, _ := json.Marshal(cfg.ConfigFields)
	var dataVolume *string
	if cfg.DataVolume != "" {
		dataVolume = &cfg.DataVolume
	}
	return r.db.Pool.QueryRow(ctx,
		`INSERT INTO game_server_configs (app_id, template_id, image, game_port, query_port, data_dir, data_volume, protocol, config_fields)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		cfg.AppID, cfg.TemplateID, cfg.Image, cfg.GamePort, cfg.QueryPort, cfg.DataDir, dataVolume, cfg.Protocol, fields,
	).Scan(&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt)
}

func (r *GameServerConfigRepo) Update(ctx context.Context, cfg *model.GameServerConfig) error {
	fields, _ := json.Marshal(cfg.ConfigFields)
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE game_server_configs SET config_fields=$2, updated_at=NOW() WHERE id=$1`,
		cfg.ID, fields,
	)
	return err
}
