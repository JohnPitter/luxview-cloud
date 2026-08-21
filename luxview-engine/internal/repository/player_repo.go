package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type PlayerRepo struct {
	db *DB
}

func NewPlayerRepo(db *DB) *PlayerRepo {
	return &PlayerRepo{db: db}
}

func (r *PlayerRepo) Create(ctx context.Context, p *model.PlayerAccount) error {
	return r.db.Pool.QueryRow(ctx,
		`INSERT INTO player_accounts (username, password_hash) VALUES ($1, $2)
		 RETURNING id, cash_points, reward_points, created_at`,
		p.Username, p.PasswordHash,
	).Scan(&p.ID, &p.CashPoints, &p.RewardPoints, &p.CreatedAt)
}

func (r *PlayerRepo) FindByUsername(ctx context.Context, username string) (*model.PlayerAccount, error) {
	return r.scanAccount(ctx,
		`SELECT id, username, password_hash, cash_points, reward_points, created_at
		 FROM player_accounts WHERE lower(username) = lower($1)`, username)
}

func (r *PlayerRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.PlayerAccount, error) {
	return r.scanAccount(ctx,
		`SELECT id, username, password_hash, cash_points, reward_points, created_at
		 FROM player_accounts WHERE id = $1`, id)
}

func (r *PlayerRepo) scanAccount(ctx context.Context, q string, arg any) (*model.PlayerAccount, error) {
	var p model.PlayerAccount
	err := r.db.Pool.QueryRow(ctx, q, arg).Scan(
		&p.ID, &p.Username, &p.PasswordHash, &p.CashPoints, &p.RewardPoints, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("player account: %w", err)
	}
	return &p, nil
}

func (r *PlayerRepo) CreateLink(ctx context.Context, link *model.PlayerGameLink) error {
	return r.db.Pool.QueryRow(ctx,
		`INSERT INTO player_game_links (player_id, app_id, template_id, in_game_nick)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		link.PlayerID, link.AppID, link.TemplateID, link.InGameNick,
	).Scan(&link.ID, &link.CreatedAt)
}

func (r *PlayerRepo) EnsureLink(ctx context.Context, link *model.PlayerGameLink) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO player_game_links (player_id, app_id, template_id, in_game_nick)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (player_id, app_id) DO UPDATE SET in_game_nick = EXCLUDED.in_game_nick, template_id = EXCLUDED.template_id`,
		link.PlayerID, link.AppID, link.TemplateID, link.InGameNick,
	)
	return err
}

func (r *PlayerRepo) ListLinks(ctx context.Context, playerID uuid.UUID) ([]model.PlayerGameLink, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, player_id, app_id, template_id, in_game_nick, created_at
		 FROM player_game_links WHERE player_id = $1 ORDER BY created_at`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PlayerGameLink
	for rows.Next() {
		var l model.PlayerGameLink
		if err := rows.Scan(&l.ID, &l.PlayerID, &l.AppID, &l.TemplateID, &l.InGameNick, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if out == nil {
		out = []model.PlayerGameLink{}
	}
	return out, rows.Err()
}

func (r *PlayerRepo) AppendLedger(ctx context.Context, playerID uuid.UUID, kind model.LedgerKind, delta int64, reason string) (*model.PlayerAccount, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	col := "cash_points"
	if kind == model.LedgerReward {
		col = "reward_points"
	}
	var p model.PlayerAccount
	q := fmt.Sprintf(`UPDATE player_accounts SET %s = %s + $2 WHERE id = $1
		RETURNING id, username, password_hash, cash_points, reward_points, created_at`, col, col)
	if err := tx.QueryRow(ctx, q, playerID, delta).Scan(
		&p.ID, &p.Username, &p.PasswordHash, &p.CashPoints, &p.RewardPoints, &p.CreatedAt); err != nil {
		return nil, err
	}
	if p.CashPoints < 0 || p.RewardPoints < 0 {
		return nil, fmt.Errorf("saldo insuficiente")
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO player_ledger (player_id, kind, delta, reason) VALUES ($1, $2, $3, $4)`,
		playerID, kind, delta, reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}
