package model

import (
	"time"

	"github.com/google/uuid"
)

type Plan struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Price               float64   `json:"price"`
	Currency            string    `json:"currency"`
	BillingCycle        string    `json:"billing_cycle"`
	MaxApps             int       `json:"max_apps"`
	MaxCPUPerApp        float64   `json:"max_cpu_per_app"`
	MaxMemoryPerApp     string    `json:"max_memory_per_app"`
	MaxDiskPerApp       string    `json:"max_disk_per_app"`
	MaxServicesPerApp   int       `json:"max_services_per_app"`
	AutoDeployEnabled   bool      `json:"auto_deploy_enabled"`
	CustomDomainEnabled bool      `json:"custom_domain_enabled"`
	PriorityBuilds      bool      `json:"priority_builds"`
	Highlighted         bool      `json:"highlighted"`
	SortOrder           int       `json:"sort_order"`
	Features            []string  `json:"features"`
	IsActive            bool      `json:"is_active"`
	IsDefault           bool      `json:"is_default"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreatePlanRequest struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Price               float64  `json:"price"`
	Currency            string   `json:"currency"`
	BillingCycle        string   `json:"billing_cycle"`
	MaxApps             int      `json:"max_apps"`
	MaxCPUPerApp        float64  `json:"max_cpu_per_app"`
	MaxMemoryPerApp     string   `json:"max_memory_per_app"`
	MaxDiskPerApp       string   `json:"max_disk_per_app"`
	MaxServicesPerApp   int      `json:"max_services_per_app"`
	AutoDeployEnabled   bool     `json:"auto_deploy_enabled"`
	CustomDomainEnabled bool     `json:"custom_domain_enabled"`
	PriorityBuilds      bool     `json:"priority_builds"`
	Highlighted         bool     `json:"highlighted"`
	SortOrder           int      `json:"sort_order"`
	Features            []string `json:"features"`
}

type UpdatePlanRequest struct {
	Name                *string  `json:"name,omitempty"`
	Description         *string  `json:"description,omitempty"`
	Price               *float64 `json:"price,omitempty"`
	Currency            *string  `json:"currency,omitempty"`
	BillingCycle        *string  `json:"billing_cycle,omitempty"`
	MaxApps             *int     `json:"max_apps,omitempty"`
	MaxCPUPerApp        *float64 `json:"max_cpu_per_app,omitempty"`
	MaxMemoryPerApp     *string  `json:"max_memory_per_app,omitempty"`
	MaxDiskPerApp       *string  `json:"max_disk_per_app,omitempty"`
	MaxServicesPerApp   *int     `json:"max_services_per_app,omitempty"`
	AutoDeployEnabled   *bool    `json:"auto_deploy_enabled,omitempty"`
	CustomDomainEnabled *bool    `json:"custom_domain_enabled,omitempty"`
	PriorityBuilds      *bool    `json:"priority_builds,omitempty"`
	Highlighted         *bool    `json:"highlighted,omitempty"`
	SortOrder           *int     `json:"sort_order,omitempty"`
	Features            []string `json:"features,omitempty"`
	IsActive            *bool    `json:"is_active,omitempty"`
	IsDefault           *bool    `json:"is_default,omitempty"`
}
