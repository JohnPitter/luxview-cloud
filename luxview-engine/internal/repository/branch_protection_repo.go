package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type BranchProtectionRepo struct {
	db *DB
}

func NewBranchProtectionRepo(db *DB) *BranchProtectionRepo {
	return &BranchProtectionRepo{db: db}
}

// Upsert creates or updates the protection rule for a (repository, branch) pair.
func (r *BranchProtectionRepo) Upsert(ctx context.Context, rule *model.BranchProtectionRule) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO branch_protection_rules
		 (repository_id, branch, require_reviews, required_approvals, dismiss_stale_reviews, require_status_checks, block_force_push)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (repository_id, branch) DO UPDATE SET
		   require_reviews = EXCLUDED.require_reviews,
		   required_approvals = EXCLUDED.required_approvals,
		   dismiss_stale_reviews = EXCLUDED.dismiss_stale_reviews,
		   require_status_checks = EXCLUDED.require_status_checks,
		   block_force_push = EXCLUDED.block_force_push,
		   updated_at = NOW()
		 RETURNING id, created_at, updated_at`,
		rule.RepositoryID, rule.Branch, rule.RequireReviews, rule.RequiredApprovals,
		rule.DismissStaleReviews, rule.RequireStatusChecks, rule.BlockForcePush,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert branch protection: %w", err)
	}
	return nil
}

func (r *BranchProtectionRepo) ListByRepository(ctx context.Context, repositoryID uuid.UUID) ([]model.BranchProtectionRule, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, repository_id, branch, require_reviews, required_approvals,
		        dismiss_stale_reviews, require_status_checks, block_force_push, created_at, updated_at
		 FROM branch_protection_rules WHERE repository_id = $1 ORDER BY branch ASC`, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list branch protection: %w", err)
	}
	defer rows.Close()

	var rules []model.BranchProtectionRule
	for rows.Next() {
		var rule model.BranchProtectionRule
		if err := rows.Scan(&rule.ID, &rule.RepositoryID, &rule.Branch, &rule.RequireReviews, &rule.RequiredApprovals,
			&rule.DismissStaleReviews, &rule.RequireStatusChecks, &rule.BlockForcePush, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan branch protection: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *BranchProtectionRepo) FindByBranch(ctx context.Context, repositoryID uuid.UUID, branch string) (*model.BranchProtectionRule, error) {
	var rule model.BranchProtectionRule
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, repository_id, branch, require_reviews, required_approvals,
		        dismiss_stale_reviews, require_status_checks, block_force_push, created_at, updated_at
		 FROM branch_protection_rules WHERE repository_id = $1 AND branch = $2`, repositoryID, branch,
	).Scan(&rule.ID, &rule.RepositoryID, &rule.Branch, &rule.RequireReviews, &rule.RequiredApprovals,
		&rule.DismissStaleReviews, &rule.RequireStatusChecks, &rule.BlockForcePush, &rule.CreatedAt, &rule.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find branch protection: %w", err)
	}
	return &rule, nil
}

func (r *BranchProtectionRepo) Delete(ctx context.Context, repositoryID uuid.UUID, branch string) error {
	res, err := r.db.Pool.Exec(ctx,
		`DELETE FROM branch_protection_rules WHERE repository_id = $1 AND branch = $2`, repositoryID, branch)
	if err != nil {
		return fmt.Errorf("delete branch protection: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("rule not found")
	}
	return nil
}
