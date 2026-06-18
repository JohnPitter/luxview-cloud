package model

import (
	"time"

	"github.com/google/uuid"
)

type BranchProtectionRule struct {
	ID                  uuid.UUID `json:"id"`
	RepositoryID        uuid.UUID `json:"repository_id"`
	Branch              string    `json:"branch"`
	RequireReviews      bool      `json:"require_reviews"`
	RequiredApprovals   int       `json:"required_approvals"`
	DismissStaleReviews bool      `json:"dismiss_stale_reviews"`
	RequireStatusChecks bool      `json:"require_status_checks"`
	BlockForcePush      bool      `json:"block_force_push"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
