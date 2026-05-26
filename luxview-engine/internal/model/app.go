package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AppStatus string

const (
	AppStatusBuilding    AppStatus = "building"
	AppStatusRunning     AppStatus = "running"
	AppStatusStopped     AppStatus = "stopped"
	AppStatusError       AppStatus = "error"
	AppStatusSleeping    AppStatus = "sleeping"
	AppStatusMaintenance AppStatus = "maintenance"
)

type AppType string

const (
	AppTypeWeb  AppType = "web"
	AppTypeGame AppType = "game"
)

type ResourceLimits struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

type App struct {
	ID               uuid.UUID         `json:"id"`
	UserID           uuid.UUID         `json:"user_id"`
	Name             string            `json:"name"`
	Subdomain        string            `json:"subdomain"`
	RepositoryID     *uuid.UUID        `json:"repository_id,omitempty"`
	RepoURL          string            `json:"repo_url"`
	RepoBranch       string            `json:"repo_branch"`
	Stack            string            `json:"stack"`
	Status           AppStatus         `json:"status"`
	AppType          AppType           `json:"app_type"`
	ContainerID      string            `json:"container_id,omitempty"`
	InternalPort     int               `json:"internal_port"`
	AssignedPort     int               `json:"assigned_port"`
	EnvVars          json.RawMessage   `json:"-"`        // encrypted, never directly exposed
	EnvVarsPlain     map[string]string `json:"env_vars"` // decrypted for API responses
	ResourceLimits   ResourceLimits    `json:"resource_limits"`
	AutoDeploy       bool              `json:"auto_deploy"`
	WebhookID        *int64            `json:"webhook_id,omitempty"`
	CustomDockerfile *string           `json:"custom_dockerfile,omitempty"`
	CustomDomain     *string           `json:"custom_domain,omitempty"`
	GameConfig       *GameServerConfig `json:"game_config,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type CreateGameServerConfig struct {
	TemplateID string `json:"template_id"`
	Image      string `json:"image"`
	GamePort   int    `json:"game_port"`
	QueryPort  int    `json:"query_port,omitempty"`
	DataDir    string `json:"data_dir,omitempty"`
	DataVolume string `json:"data_volume,omitempty"` // leave empty to auto-generate; set to existing volume name to preserve data
}

type CreateAppRequest struct {
	Name         string                  `json:"name"`
	Subdomain    string                  `json:"subdomain"`
	RepositoryID *uuid.UUID              `json:"repository_id,omitempty"`
	RepoURL      string                  `json:"repo_url"`
	RepoBranch   string                  `json:"repo_branch"`
	EnvVars      map[string]string       `json:"env_vars"`
	AutoDeploy   *bool                   `json:"auto_deploy"`
	AppType      AppType                 `json:"app_type,omitempty"`
	GameConfig   *CreateGameServerConfig `json:"game_config,omitempty"`
}

type UpdateAppRequest struct {
	Name           *string           `json:"name,omitempty"`
	RepoBranch     *string           `json:"repo_branch,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	ResourceLimits *ResourceLimits   `json:"resource_limits,omitempty"`
	AutoDeploy     *bool             `json:"auto_deploy,omitempty"`
	CustomDomain   *string           `json:"custom_domain,omitempty"`
}

// ReservedSubdomains that cannot be used by users.
var ReservedSubdomains = map[string]bool{
	"api":       true,
	"www":       true,
	"admin":     true,
	"mail":      true,
	"ftp":       true,
	"dashboard": true,
	"app":       true,
	"static":    true,
	"assets":    true,
	"ws":        true,
	"status":    true,
	"docs":      true,
	"help":      true,
	"support":   true,
	"blog":      true,
}
