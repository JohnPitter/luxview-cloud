package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByGitHubID(ctx context.Context, githubID int64) (*model.User, error) {
	var u model.User
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, github_id, username, email, avatar_url, github_token, role, created_at, last_login_at, plan_id
		 FROM users WHERE github_id = $1`, githubID,
	).Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.GitHubToken, &u.Role, &u.CreatedAt, &u.LastLoginAt, &u.PlanID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by github_id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var u model.User
	// Nullable plan columns for LEFT JOIN
	var planID *uuid.UUID
	var pName, pDescription, pCurrency, pBillingCycle, pMaxMemory, pMaxDisk *string
	var pPrice *float64
	var pMaxApps, pMaxServicesPerApp, pSortOrder *int
	var pMaxCPU *float64
	var pAutoDeploy, pCustomDomain, pPriorityBuilds, pHighlighted, pIsActive, pIsDefault *bool
	var pFeatures json.RawMessage
	var pCreatedAt, pUpdatedAt *time.Time

	err := r.db.Pool.QueryRow(ctx,
		`SELECT u.id, u.github_id, u.username, u.email, u.avatar_url, u.github_token, u.role, u.created_at, u.last_login_at, u.plan_id,
		        p.id, p.name, p.description, p.price, p.currency, p.billing_cycle, p.max_apps, p.max_cpu_per_app,
		        p.max_memory_per_app, p.max_disk_per_app, p.max_services_per_app, p.auto_deploy_enabled,
		        p.custom_domain_enabled, p.priority_builds, p.highlighted, p.sort_order, p.features,
		        p.is_active, p.is_default, p.created_at, p.updated_at
		 FROM users u LEFT JOIN plans p ON u.plan_id = p.id
		 WHERE u.id = $1`, id,
	).Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.GitHubToken, &u.Role, &u.CreatedAt, &u.LastLoginAt, &u.PlanID,
		&planID, &pName, &pDescription, &pPrice, &pCurrency, &pBillingCycle, &pMaxApps, &pMaxCPU,
		&pMaxMemory, &pMaxDisk, &pMaxServicesPerApp, &pAutoDeploy,
		&pCustomDomain, &pPriorityBuilds, &pHighlighted, &pSortOrder, &pFeatures,
		&pIsActive, &pIsDefault, &pCreatedAt, &pUpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	if planID != nil {
		plan := &model.Plan{
			ID:                  *planID,
			Name:                derefStr(pName),
			Description:         derefStr(pDescription),
			Price:               derefFloat(pPrice),
			Currency:            derefStr(pCurrency),
			BillingCycle:        derefStr(pBillingCycle),
			MaxApps:             derefInt(pMaxApps),
			MaxCPUPerApp:        derefFloat(pMaxCPU),
			MaxMemoryPerApp:     derefStr(pMaxMemory),
			MaxDiskPerApp:       derefStr(pMaxDisk),
			MaxServicesPerApp:   derefInt(pMaxServicesPerApp),
			AutoDeployEnabled:   derefBool(pAutoDeploy),
			CustomDomainEnabled: derefBool(pCustomDomain),
			PriorityBuilds:      derefBool(pPriorityBuilds),
			Highlighted:         derefBool(pHighlighted),
			SortOrder:           derefInt(pSortOrder),
			IsActive:            derefBool(pIsActive),
			IsDefault:           derefBool(pIsDefault),
		}
		if pCreatedAt != nil {
			plan.CreatedAt = *pCreatedAt
		}
		if pUpdatedAt != nil {
			plan.UpdatedAt = *pUpdatedAt
		}
		_ = json.Unmarshal(pFeatures, &plan.Features)
		if plan.Features == nil {
			plan.Features = []string{}
		}
		u.Plan = plan
	}

	return &u, nil
}

func (r *UserRepo) Upsert(ctx context.Context, u *model.User) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO users (github_id, username, email, avatar_url, github_token, role, last_login_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (github_id) DO UPDATE SET
		   username = EXCLUDED.username,
		   email = EXCLUDED.email,
		   avatar_url = EXCLUDED.avatar_url,
		   github_token = EXCLUDED.github_token,
		   last_login_at = NOW()
		 RETURNING id`,
		u.GitHubID, u.Username, u.Email, u.AvatarURL, u.GitHubToken, u.Role,
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	// Re-read the ID after upsert
	if err := r.db.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE github_id = $1`, u.GitHubID,
	).Scan(&u.ID); err != nil {
		return fmt.Errorf("read user id after upsert: %w", err)
	}

	// Assign default plan if user has no plan
	var defaultPlanID uuid.UUID
	err = r.db.Pool.QueryRow(ctx,
		`SELECT id FROM plans WHERE is_default = true AND is_active = true LIMIT 1`,
	).Scan(&defaultPlanID)
	if err == nil {
		_, _ = r.db.Pool.Exec(ctx,
			`UPDATE users SET plan_id = $1 WHERE id = $2 AND plan_id IS NULL`,
			defaultPlanID, u.ID)
	}

	return nil
}

func (r *UserRepo) ListAll(ctx context.Context, limit, offset int) ([]model.User, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, github_id, username, email, avatar_url, role, created_at, last_login_at, plan_id
		 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.GitHubID, &u.Username, &u.Email, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.LastLoginAt, &u.PlanID); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role model.UserRole) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE users SET role = $1 WHERE id = $2`, role, id)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	return nil
}

func (r *UserRepo) UpdatePlanID(ctx context.Context, userID uuid.UUID, planID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE users SET plan_id = $1 WHERE id = $2`, planID, userID)
	if err != nil {
		return fmt.Errorf("update user plan_id: %w", err)
	}
	return nil
}

// Helper functions for nullable scan fields
func derefStr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

func derefFloat(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}

func derefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}

func derefBool(p *bool) bool {
	if p != nil {
		return *p
	}
	return false
}
