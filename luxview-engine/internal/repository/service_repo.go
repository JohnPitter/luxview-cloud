package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type ServiceRepo struct {
	db *DB
}

func NewServiceRepo(db *DB) *ServiceRepo {
	return &ServiceRepo{db: db}
}

func (r *ServiceRepo) Create(ctx context.Context, s *model.AppService) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO app_services (app_id, service_type, db_name, credentials)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		s.AppID, s.ServiceType, s.DBName, s.Credentials,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	return nil
}

func (r *ServiceRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.AppService, error) {
	var s model.AppService
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, service_type, db_name, credentials, created_at
		 FROM app_services WHERE id = $1`, id,
	).Scan(&s.ID, &s.AppID, &s.ServiceType, &s.DBName, &s.Credentials, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find service: %w", err)
	}
	return &s, nil
}

func (r *ServiceRepo) ListByAppID(ctx context.Context, appID uuid.UUID) ([]model.AppService, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, app_id, service_type, db_name, credentials, created_at
		 FROM app_services WHERE app_id = $1 ORDER BY created_at`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.AppService
	for rows.Next() {
		var s model.AppService
		if err := rows.Scan(&s.ID, &s.AppID, &s.ServiceType, &s.DBName, &s.Credentials, &s.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, nil
}

func (r *ServiceRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.AppService, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT s.id, s.app_id, s.service_type, s.db_name, s.credentials, s.created_at
		 FROM app_services s
		 JOIN apps a ON a.id = s.app_id
		 WHERE a.user_id = $1
		 ORDER BY s.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []model.AppService
	for rows.Next() {
		var s model.AppService
		if err := rows.Scan(&s.ID, &s.AppID, &s.ServiceType, &s.DBName, &s.Credentials, &s.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, nil
}

func (r *ServiceRepo) FindByAppAndType(ctx context.Context, appID uuid.UUID, serviceType model.ServiceType) (*model.AppService, error) {
	var s model.AppService
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, service_type, db_name, credentials, created_at
		 FROM app_services WHERE app_id = $1 AND service_type = $2`, appID, serviceType,
	).Scan(&s.ID, &s.AppID, &s.ServiceType, &s.DBName, &s.Credentials, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceRepo) CountByType(ctx context.Context, serviceType model.ServiceType) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM app_services WHERE service_type = $1`, serviceType,
	).Scan(&count)
	return count, err
}

func (r *ServiceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM app_services WHERE id = $1`, id)
	return err
}
