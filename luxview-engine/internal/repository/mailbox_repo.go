package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type MailboxRepo struct {
	db *DB
}

func NewMailboxRepo(db *DB) *MailboxRepo {
	return &MailboxRepo{db: db}
}

func (r *MailboxRepo) Create(ctx context.Context, m *model.Mailbox) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO mailboxes (service_id, address)
		 VALUES ($1, $2)
		 RETURNING id, created_at`,
		m.ServiceID, m.Address,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return fmt.Errorf("create mailbox: %w", err)
	}
	return nil
}

func (r *MailboxRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Mailbox, error) {
	var m model.Mailbox
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, service_id, address, created_at
		 FROM mailboxes WHERE id = $1`, id,
	).Scan(&m.ID, &m.ServiceID, &m.Address, &m.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find mailbox: %w", err)
	}
	return &m, nil
}

func (r *MailboxRepo) ListByServiceID(ctx context.Context, serviceID uuid.UUID) ([]model.Mailbox, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, service_id, address, created_at
		 FROM mailboxes WHERE service_id = $1 ORDER BY created_at`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list mailboxes: %w", err)
	}
	defer rows.Close()

	var mailboxes []model.Mailbox
	for rows.Next() {
		var m model.Mailbox
		if err := rows.Scan(&m.ID, &m.ServiceID, &m.Address, &m.CreatedAt); err != nil {
			return nil, err
		}
		mailboxes = append(mailboxes, m)
	}
	return mailboxes, nil
}

func (r *MailboxRepo) CountByServiceID(ctx context.Context, serviceID uuid.UUID) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mailboxes WHERE service_id = $1`, serviceID,
	).Scan(&count)
	return count, err
}

func (r *MailboxRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM mailboxes WHERE id = $1`, id)
	return err
}

func (r *MailboxRepo) DeleteByServiceID(ctx context.Context, serviceID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM mailboxes WHERE service_id = $1`, serviceID)
	return err
}
