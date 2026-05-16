package model

import (
	"time"

	"github.com/google/uuid"
)

type ActionStatus string

const (
	ActionQueued    ActionStatus = "queued"
	ActionRunning   ActionStatus = "running"
	ActionSuccess   ActionStatus = "success"
	ActionFailed    ActionStatus = "failed"
	ActionCancelled ActionStatus = "cancelled"
	ActionSkipped   ActionStatus = "skipped"
)

type ActionRun struct {
	ID           uuid.UUID    `json:"id"`
	AppID        uuid.UUID    `json:"app_id"`
	Workflow     string       `json:"workflow"`
	WorkflowPath string       `json:"workflow_path"`
	Trigger      string       `json:"trigger"`
	Branch       string       `json:"branch"`
	CommitSHA    string       `json:"commit_sha"`
	Status       ActionStatus `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	FinishedAt   *time.Time   `json:"finished_at,omitempty"`
}

type ActionJob struct {
	ID         uuid.UUID    `json:"id"`
	RunID      uuid.UUID    `json:"run_id"`
	Name       string       `json:"name"`
	Image      string       `json:"image"`
	Status     ActionStatus `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
}

type ActionStep struct {
	ID         uuid.UUID         `json:"id"`
	JobID      uuid.UUID         `json:"job_id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Command    string            `json:"command"`
	Uses       string            `json:"uses"`
	Inputs     map[string]string `json:"inputs,omitempty"`
	Status     ActionStatus      `json:"status"`
	ExitCode   int               `json:"exit_code"`
	Log        string            `json:"log,omitempty"`
	Position   int               `json:"position"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
}

type ActionRunDetail struct {
	Run   ActionRun    `json:"run"`
	Jobs  []ActionJob  `json:"jobs"`
	Steps []ActionStep `json:"steps"`
}

type ActionArtifact struct {
	ID        uuid.UUID `json:"id"`
	RunID     uuid.UUID `json:"run_id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type ActionSecret struct {
	ID        uuid.UUID `json:"id"`
	AppID     uuid.UUID `json:"app_id"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
