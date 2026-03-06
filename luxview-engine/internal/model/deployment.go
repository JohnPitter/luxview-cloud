package model

import (
	"time"

	"github.com/google/uuid"
)

type DeploymentStatus string

const (
	DeployPending    DeploymentStatus = "pending"
	DeployBuilding   DeploymentStatus = "building"
	DeployDeploying  DeploymentStatus = "deploying"
	DeployLive       DeploymentStatus = "live"
	DeployFailed     DeploymentStatus = "failed"
	DeployRolledBack DeploymentStatus = "rolled_back"
)

type Deployment struct {
	ID            uuid.UUID        `json:"id"`
	AppID         uuid.UUID        `json:"app_id"`
	CommitSHA     string           `json:"commit_sha"`
	CommitMessage string           `json:"commit_message"`
	Status        DeploymentStatus `json:"status"`
	BuildLog      string           `json:"build_log,omitempty"`
	DurationMs    int              `json:"duration_ms"`
	ImageTag      string           `json:"image_tag"`
	CreatedAt     time.Time        `json:"created_at"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
}
