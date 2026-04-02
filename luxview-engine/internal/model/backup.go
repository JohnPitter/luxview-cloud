package model

import (
	"time"

	"github.com/google/uuid"
)

type BackupStatus string

const (
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusRestoring BackupStatus = "restoring"
)

type BackupTrigger string

const (
	BackupTriggerScheduled BackupTrigger = "scheduled"
	BackupTriggerManual    BackupTrigger = "manual"
)

type BackupDatabase string

const (
	DBPGPlatform  BackupDatabase = "pg-platform"
	DBPGShared    BackupDatabase = "pg-shared"
	DBMongoShared BackupDatabase = "mongo-shared"
	DBRedisShared BackupDatabase = "redis-shared"
)

var validDatabases = map[string]bool{
	"pg-platform":  true,
	"pg-shared":    true,
	"mongo-shared": true,
	"redis-shared": true,
}

var validSchedules = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
}

var validRetentions = map[int]bool{
	7: true, 14: true, 30: true, 60: true,
}

func IsValidDatabase(s string) bool { return validDatabases[s] }
func IsValidSchedule(s string) bool { return validSchedules[s] }
func IsValidRetention(d int) bool   { return validRetentions[d] }

type Backup struct {
	ID          uuid.UUID     `json:"id"`
	Databases   []string      `json:"databases"`
	Status      BackupStatus  `json:"status"`
	Trigger     BackupTrigger `json:"trigger"`
	FilePath    string        `json:"-"`
	FileSize    int64         `json:"file_size"`
	DurationMs  int           `json:"duration_ms"`
	Error       string        `json:"error,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	CreatedBy   *uuid.UUID    `json:"created_by,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

type BackupSettings struct {
	Enabled       bool     `json:"enabled"`
	Schedule      string   `json:"schedule"`
	RetentionDays int      `json:"retention_days"`
	Databases     []string `json:"databases"`
}
