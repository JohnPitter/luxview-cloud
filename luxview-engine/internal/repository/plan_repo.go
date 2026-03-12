package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type PlanRepo struct {
	db *DB
}

func NewPlanRepo(db *DB) *PlanRepo {
	return &PlanRepo{db: db}
}

func (r *PlanRepo) Create(ctx context.Context, plan *model.Plan) error {
	featuresJSON, err := json.Marshal(plan.Features)
	if err != nil {
		featuresJSON = []byte(`[]`)
	}

	err = r.db.Pool.QueryRow(ctx,
		`INSERT INTO plans (name, description, price, currency, billing_cycle, max_apps, max_cpu_per_app,
		 max_memory_per_app, max_disk_per_app, max_services_per_app, max_mailboxes_per_app, max_mailbox_storage, auto_deploy_enabled, custom_domain_enabled,
		 priority_builds, highlighted, sort_order, features, is_active, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		 RETURNING id, created_at, updated_at`,
		plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingCycle,
		plan.MaxApps, plan.MaxCPUPerApp, plan.MaxMemoryPerApp, plan.MaxDiskPerApp,
		plan.MaxServicesPerApp, plan.MaxMailboxesPerApp, plan.MaxMailboxStorage,
		plan.AutoDeployEnabled, plan.CustomDomainEnabled,
		plan.PriorityBuilds, plan.Highlighted, plan.SortOrder, featuresJSON,
		plan.IsActive, plan.IsDefault,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

func (r *PlanRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Plan, error) {
	var plan model.Plan
	var featuresRaw json.RawMessage
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, description, price, currency, billing_cycle, max_apps, max_cpu_per_app,
		 max_memory_per_app, max_disk_per_app, max_services_per_app, max_mailboxes_per_app, max_mailbox_storage, auto_deploy_enabled, custom_domain_enabled,
		 priority_builds, highlighted, sort_order, features, is_active, is_default, created_at, updated_at
		 FROM plans WHERE id = $1`, id,
	).Scan(&plan.ID, &plan.Name, &plan.Description, &plan.Price, &plan.Currency, &plan.BillingCycle,
		&plan.MaxApps, &plan.MaxCPUPerApp, &plan.MaxMemoryPerApp, &plan.MaxDiskPerApp,
		&plan.MaxServicesPerApp, &plan.MaxMailboxesPerApp, &plan.MaxMailboxStorage, &plan.AutoDeployEnabled, &plan.CustomDomainEnabled,
		&plan.PriorityBuilds, &plan.Highlighted, &plan.SortOrder, &featuresRaw,
		&plan.IsActive, &plan.IsDefault, &plan.CreatedAt, &plan.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find plan by id: %w", err)
	}
	_ = json.Unmarshal(featuresRaw, &plan.Features)
	if plan.Features == nil {
		plan.Features = []string{}
	}
	return &plan, nil
}

func (r *PlanRepo) Update(ctx context.Context, plan *model.Plan) error {
	featuresJSON, err := json.Marshal(plan.Features)
	if err != nil {
		featuresJSON = []byte(`[]`)
	}

	_, err = r.db.Pool.Exec(ctx,
		`UPDATE plans SET name=$2, description=$3, price=$4, currency=$5, billing_cycle=$6,
		 max_apps=$7, max_cpu_per_app=$8, max_memory_per_app=$9, max_disk_per_app=$10,
		 max_services_per_app=$11, max_mailboxes_per_app=$12, max_mailbox_storage=$13,
		 auto_deploy_enabled=$14, custom_domain_enabled=$15,
		 priority_builds=$16, highlighted=$17, sort_order=$18, features=$19,
		 is_active=$20, is_default=$21, updated_at=NOW()
		 WHERE id=$1`,
		plan.ID, plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingCycle,
		plan.MaxApps, plan.MaxCPUPerApp, plan.MaxMemoryPerApp, plan.MaxDiskPerApp,
		plan.MaxServicesPerApp, plan.MaxMailboxesPerApp, plan.MaxMailboxStorage,
		plan.AutoDeployEnabled, plan.CustomDomainEnabled,
		plan.PriorityBuilds, plan.Highlighted, plan.SortOrder, featuresJSON,
		plan.IsActive, plan.IsDefault,
	)
	if err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	return nil
}

func (r *PlanRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE plans SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("soft delete plan: %w", err)
	}
	return nil
}

func (r *PlanRepo) ListAll(ctx context.Context) ([]model.Plan, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, description, price, currency, billing_cycle, max_apps, max_cpu_per_app,
		 max_memory_per_app, max_disk_per_app, max_services_per_app, max_mailboxes_per_app, max_mailbox_storage, auto_deploy_enabled, custom_domain_enabled,
		 priority_builds, highlighted, sort_order, features, is_active, is_default, created_at, updated_at
		 FROM plans ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list all plans: %w", err)
	}
	defer rows.Close()

	return scanPlans(rows)
}

func (r *PlanRepo) ListActive(ctx context.Context) ([]model.Plan, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, description, price, currency, billing_cycle, max_apps, max_cpu_per_app,
		 max_memory_per_app, max_disk_per_app, max_services_per_app, max_mailboxes_per_app, max_mailbox_storage, auto_deploy_enabled, custom_domain_enabled,
		 priority_builds, highlighted, sort_order, features, is_active, is_default, created_at, updated_at
		 FROM plans WHERE is_active = true ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list active plans: %w", err)
	}
	defer rows.Close()

	return scanPlans(rows)
}

func (r *PlanRepo) SetDefault(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE plans SET is_default = false WHERE is_default = true`); err != nil {
		return fmt.Errorf("unset previous default: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE plans SET is_default = true, updated_at = NOW() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("set new default: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *PlanRepo) FindDefault(ctx context.Context) (*model.Plan, error) {
	var plan model.Plan
	var featuresRaw json.RawMessage
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, name, description, price, currency, billing_cycle, max_apps, max_cpu_per_app,
		 max_memory_per_app, max_disk_per_app, max_services_per_app, max_mailboxes_per_app, max_mailbox_storage, auto_deploy_enabled, custom_domain_enabled,
		 priority_builds, highlighted, sort_order, features, is_active, is_default, created_at, updated_at
		 FROM plans WHERE is_default = true AND is_active = true LIMIT 1`,
	).Scan(&plan.ID, &plan.Name, &plan.Description, &plan.Price, &plan.Currency, &plan.BillingCycle,
		&plan.MaxApps, &plan.MaxCPUPerApp, &plan.MaxMemoryPerApp, &plan.MaxDiskPerApp,
		&plan.MaxServicesPerApp, &plan.MaxMailboxesPerApp, &plan.MaxMailboxStorage, &plan.AutoDeployEnabled, &plan.CustomDomainEnabled,
		&plan.PriorityBuilds, &plan.Highlighted, &plan.SortOrder, &featuresRaw,
		&plan.IsActive, &plan.IsDefault, &plan.CreatedAt, &plan.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find default plan: %w", err)
	}
	_ = json.Unmarshal(featuresRaw, &plan.Features)
	if plan.Features == nil {
		plan.Features = []string{}
	}
	return &plan, nil
}

func scanPlans(rows pgx.Rows) ([]model.Plan, error) {
	var plans []model.Plan
	for rows.Next() {
		var plan model.Plan
		var featuresRaw json.RawMessage
		if err := rows.Scan(&plan.ID, &plan.Name, &plan.Description, &plan.Price, &plan.Currency,
			&plan.BillingCycle, &plan.MaxApps, &plan.MaxCPUPerApp, &plan.MaxMemoryPerApp,
			&plan.MaxDiskPerApp, &plan.MaxServicesPerApp, &plan.AutoDeployEnabled,
			&plan.CustomDomainEnabled, &plan.PriorityBuilds, &plan.Highlighted,
			&plan.SortOrder, &featuresRaw, &plan.IsActive, &plan.IsDefault,
			&plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(featuresRaw, &plan.Features)
		if plan.Features == nil {
			plan.Features = []string{}
		}
		plans = append(plans, plan)
	}
	return plans, nil
}
