package model

import (
	"time"

	"github.com/google/uuid"
)

type RepositoryVisibility string

const (
	RepositoryVisibilityPrivate RepositoryVisibility = "private"
	RepositoryVisibilityPublic  RepositoryVisibility = "public"
)

type RepositoryRemoteMode string

const (
	RepositoryRemoteModeBackup RepositoryRemoteMode = "backup"
	RepositoryRemoteModeMirror RepositoryRemoteMode = "mirror"
)

type RepositorySyncStatus string

const (
	RepositorySyncStatusPending RepositorySyncStatus = "pending"
	RepositorySyncStatusSuccess RepositorySyncStatus = "success"
	RepositorySyncStatusFailed  RepositorySyncStatus = "failed"
)

type Repository struct {
	ID            uuid.UUID            `json:"id"`
	UserID        uuid.UUID            `json:"user_id"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	DefaultBranch string               `json:"default_branch"`
	StoragePath   string               `json:"-"`
	Visibility    RepositoryVisibility `json:"visibility"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type RepositoryRemote struct {
	ID             uuid.UUID             `json:"id"`
	RepositoryID   uuid.UUID             `json:"repository_id"`
	Provider       string                `json:"provider"`
	RemoteURL      string                `json:"remote_url"`
	Mode           RepositoryRemoteMode  `json:"mode"`
	LastSyncAt     *time.Time            `json:"last_sync_at,omitempty"`
	LastSyncStatus *RepositorySyncStatus `json:"last_sync_status,omitempty"`
	LastSyncError  string                `json:"last_sync_error,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
}

type CheckoutResult struct {
	RepositoryID uuid.UUID `json:"repository_id"`
	Ref          string    `json:"ref"`
	CommitSHA    string    `json:"commit_sha"`
	WorkDir      string    `json:"work_dir"`
}
