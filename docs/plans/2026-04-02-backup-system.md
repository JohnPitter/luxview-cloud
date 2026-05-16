# Database Backup System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a managed backup service with API + dashboard UI that allows admins to configure, schedule, trigger, restore, download, and monitor database backups.

**Architecture:** BackupService in the engine manages backup execution via `os/exec` + `docker exec` on database containers. A scheduler goroutine checks every 60s if a backup is due. BackupRepo stores backup metadata in a `backups` table. BackupHandler exposes REST endpoints. Frontend adds a `/dashboard/backups` admin page.

**Tech Stack:** Go (chi router, pgx/v5, zerolog), React (TypeScript, Tailwind, Zustand, Lucide icons, react-i18next), PostgreSQL, Docker exec for dump/restore.

**Spec:** `docs/specs/2026-04-02-backup-system-design.md`

---

## File Structure

### Backend (luxview-engine)

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/model/backup.go` | Backup struct, status/trigger constants |
| Create | `internal/model/backup_test.go` | Unit tests for model validation |
| Create | `internal/repository/backup_repo.go` | CRUD for backups table |
| Create | `internal/repository/backup_repo_test.go` | Unit tests for repo (mocked DB) |
| Create | `internal/service/backup_service.go` | Execution, restore, cleanup, scheduler |
| Create | `internal/service/backup_service_test.go` | Unit tests for service logic |
| Create | `internal/api/handlers/backup_handler.go` | HTTP endpoints |
| Create | `internal/api/handlers/backup_handler_test.go` | Unit tests for handlers |
| Create | `internal/worker/backup_worker.go` | Scheduler goroutine |
| Create | `internal/worker/backup_worker_test.go` | Unit tests for scheduler |
| Modify | `internal/repository/db.go` | Add backups table migration |
| Modify | `internal/config/config.go` | Add BackupDir config field |
| Modify | `internal/api/router.go` | Register backup routes + add BackupSvc to Deps |
| Modify | `cmd/engine/main.go` | Wire BackupService + BackupWorker |

### Frontend (luxview-dashboard)

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `src/api/backups.ts` | API client for backup endpoints |
| Create | `src/pages/Backups.tsx` | Backup management page |
| Modify | `src/App.tsx` | Add route `/dashboard/backups` |
| Modify | `src/components/layout/Sidebar.tsx` | Add Backups sidebar item |
| Modify | `src/i18n/locales/en.json` | English translations |
| Modify | `src/i18n/locales/pt-BR.json` | Portuguese translations |
| Modify | `src/i18n/locales/es.json` | Spanish translations |

---

## Task 1: Backup Model + Constants

**Files:**
- Create: `luxview-engine/internal/model/backup.go`
- Create: `luxview-engine/internal/model/backup_test.go`

- [ ] **Step 1: Write failing tests for backup model**

```go
// internal/model/backup_test.go
package model

import "testing"

func TestBackupStatusConstants(t *testing.T) {
	tests := []struct {
		status BackupStatus
		want   string
	}{
		{BackupStatusRunning, "running"},
		{BackupStatusCompleted, "completed"},
		{BackupStatusFailed, "failed"},
		{BackupStatusRestoring, "restoring"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("BackupStatus = %q, want %q", tt.status, tt.want)
		}
	}
}

func TestBackupTriggerConstants(t *testing.T) {
	tests := []struct {
		trigger BackupTrigger
		want    string
	}{
		{BackupTriggerScheduled, "scheduled"},
		{BackupTriggerManual, "manual"},
	}
	for _, tt := range tests {
		if string(tt.trigger) != tt.want {
			t.Errorf("BackupTrigger = %q, want %q", tt.trigger, tt.want)
		}
	}
}

func TestBackupDatabaseConstants(t *testing.T) {
	tests := []struct {
		db   BackupDatabase
		want string
	}{
		{DBPGPlatform, "pg-platform"},
		{DBPGShared, "pg-shared"},
		{DBMongoShared, "mongo-shared"},
		{DBRedisShared, "redis-shared"},
	}
	for _, tt := range tests {
		if string(tt.db) != tt.want {
			t.Errorf("BackupDatabase = %q, want %q", tt.db, tt.want)
		}
	}
}

func TestIsValidDatabase(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"pg-platform", true},
		{"pg-shared", true},
		{"mongo-shared", true},
		{"redis-shared", true},
		{"mysql", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidDatabase(tt.input); got != tt.valid {
			t.Errorf("IsValidDatabase(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidSchedule(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"daily", true},
		{"weekly", true},
		{"monthly", true},
		{"hourly", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidSchedule(tt.input); got != tt.valid {
			t.Errorf("IsValidSchedule(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidRetention(t *testing.T) {
	tests := []struct {
		input int
		valid bool
	}{
		{7, true},
		{14, true},
		{30, true},
		{60, true},
		{0, false},
		{15, false},
		{-1, false},
	}
	for _, tt := range tests {
		if got := IsValidRetention(tt.input); got != tt.valid {
			t.Errorf("IsValidRetention(%d) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd luxview-engine && go test ./internal/model/ -run TestBackup -v`
Expected: FAIL — types and functions not defined

- [ ] **Step 3: Implement the backup model**

```go
// internal/model/backup.go
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

func IsValidDatabase(s string) bool  { return validDatabases[s] }
func IsValidSchedule(s string) bool  { return validSchedules[s] }
func IsValidRetention(d int) bool    { return validRetentions[d] }

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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd luxview-engine && go test ./internal/model/ -run TestBackup -v`
Expected: PASS — all 6 tests green

- [ ] **Step 5: Commit**

```bash
git add luxview-engine/internal/model/backup.go luxview-engine/internal/model/backup_test.go
git commit -m "feat(backup): add backup model and constants with tests"
```

---

## Task 2: Backup Repository

**Files:**
- Create: `luxview-engine/internal/repository/backup_repo.go`
- Create: `luxview-engine/internal/repository/backup_repo_test.go`
- Modify: `luxview-engine/internal/repository/db.go` (add migration)

- [ ] **Step 1: Add backups table migration to db.go**

In `internal/repository/db.go`, append these to the `migrations` slice (before the closing `}`):

```go
		`CREATE TABLE IF NOT EXISTS backups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			databases TEXT[] NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			trigger TEXT NOT NULL,
			file_path TEXT NOT NULL DEFAULT '',
			file_size BIGINT NOT NULL DEFAULT 0,
			duration_ms INT NOT NULL DEFAULT 0,
			error TEXT,
			started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			created_by UUID REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_backups_status ON backups(status)`,
		`CREATE INDEX IF NOT EXISTS idx_backups_started_at ON backups(started_at DESC)`,
```

- [ ] **Step 2: Write failing tests for backup repo**

```go
// internal/repository/backup_repo_test.go
package repository

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestNewBackupRepo(t *testing.T) {
	repo := NewBackupRepo(nil)
	if repo == nil {
		t.Fatal("NewBackupRepo returned nil")
	}
	if repo.db != nil {
		t.Fatal("expected nil db in test repo")
	}
}

func TestBackupRepoStructHasMethods(t *testing.T) {
	// Compile-time check that BackupRepo has the expected interface
	var repo *BackupRepo
	_ = repo

	// These will fail to compile if methods don't exist
	type backupRepoInterface interface {
		Create(ctx interface{}, b *model.Backup) error
		FindByID(ctx interface{}, id interface{}) (*model.Backup, error)
		List(ctx interface{}, limit, offset int) ([]model.Backup, int, error)
		UpdateStatus(ctx interface{}, id interface{}, status model.BackupStatus, err string, fileSize int64, durationMs int) error
		Delete(ctx interface{}, id interface{}) error
		DeleteOlderThan(ctx interface{}, cutoff interface{}) (int64, error)
		FindLatest(ctx interface{}) (*model.Backup, error)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd luxview-engine && go test ./internal/repository/ -run TestBackupRepo -v`
Expected: FAIL — BackupRepo type not defined

- [ ] **Step 4: Implement backup repo**

```go
// internal/repository/backup_repo.go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/luxview/engine/internal/model"
)

type BackupRepo struct {
	db *DB
}

func NewBackupRepo(db *DB) *BackupRepo {
	return &BackupRepo{db: db}
}

func (r *BackupRepo) Create(ctx context.Context, b *model.Backup) error {
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO backups (databases, status, trigger, file_path, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, started_at, created_at`,
		b.Databases, b.Status, b.Trigger, b.FilePath, b.CreatedBy,
	).Scan(&b.ID, &b.StartedAt, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	return nil
}

func (r *BackupRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Backup, error) {
	var b model.Backup
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
		&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find backup by id: %w", err)
	}
	return &b, nil
}

func (r *BackupRepo) List(ctx context.Context, limit, offset int) ([]model.Backup, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM backups`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count backups: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups ORDER BY started_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		if err := rows.Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
			&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, total, nil
}

func (r *BackupRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.BackupStatus, errMsg string, fileSize int64, durationMs int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE backups SET status = $1, error = $2, file_size = $3, duration_ms = $4, completed_at = NOW()
		 WHERE id = $5`,
		status, errMsg, fileSize, durationMs, id)
	if err != nil {
		return fmt.Errorf("update backup status: %w", err)
	}
	return nil
}

func (r *BackupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	return nil
}

func (r *BackupRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM backups WHERE started_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old backups: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *BackupRepo) FindLatest(ctx context.Context) (*model.Backup, error) {
	var b model.Backup
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, databases, status, trigger, file_path, file_size, duration_ms,
		        error, started_at, completed_at, created_by, created_at
		 FROM backups ORDER BY started_at DESC LIMIT 1`,
	).Scan(&b.ID, &b.Databases, &b.Status, &b.Trigger, &b.FilePath, &b.FileSize,
		&b.DurationMs, &b.Error, &b.StartedAt, &b.CompletedAt, &b.CreatedBy, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest backup: %w", err)
	}
	return &b, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd luxview-engine && go test ./internal/repository/ -run TestBackupRepo -v`
Expected: PASS

- [ ] **Step 6: Verify build**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add luxview-engine/internal/repository/backup_repo.go luxview-engine/internal/repository/backup_repo_test.go luxview-engine/internal/repository/db.go
git commit -m "feat(backup): add backup repository with migration and tests"
```

---

## Task 3: Config — Add BackupDir

**Files:**
- Modify: `luxview-engine/internal/config/config.go`

- [ ] **Step 1: Add BackupDir to Config struct**

In `internal/config/config.go`, add to the `Config` struct (after `MailContainerName`):

```go
	BackupDir string // base directory for backup files
```

And in `Load()`, add to the initialization (after `MailContainerName` line):

```go
		BackupDir: envStr("BACKUP_DIR", "/backups"),
```

- [ ] **Step 2: Verify build**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add luxview-engine/internal/config/config.go
git commit -m "feat(backup): add BackupDir config field"
```

---

## Task 4: Backup Service — Core Execution Logic

**Files:**
- Create: `luxview-engine/internal/service/backup_service.go`
- Create: `luxview-engine/internal/service/backup_service_test.go`

- [ ] **Step 1: Write failing tests for backup service**

```go
// internal/service/backup_service_test.go
package service

import (
	"testing"
	"time"

	"github.com/luxview/engine/internal/model"
)

func TestBuildBackupDirName(t *testing.T) {
	ts := time.Date(2026, 4, 2, 3, 0, 0, 0, time.UTC)

	got := buildBackupDirName(ts, model.BackupTriggerScheduled)
	want := "2026-04-02_030000_scheduled"
	if got != want {
		t.Errorf("buildBackupDirName() = %q, want %q", got, want)
	}

	got = buildBackupDirName(ts, model.BackupTriggerManual)
	want = "2026-04-02_030000_manual"
	if got != want {
		t.Errorf("buildBackupDirName() = %q, want %q", got, want)
	}
}

func TestDumpCommand(t *testing.T) {
	tests := []struct {
		db      string
		wantBin string
	}{
		{"pg-platform", "docker"},
		{"pg-shared", "docker"},
		{"mongo-shared", "docker"},
		{"redis-shared", "docker"},
	}
	for _, tt := range tests {
		cmd := dumpCommand(tt.db, "/backups/test", testContainerConfig())
		if cmd == nil {
			t.Fatalf("dumpCommand(%q) returned nil", tt.db)
		}
		if cmd.Path == "" {
			t.Errorf("dumpCommand(%q) has empty Path", tt.db)
		}
	}
}

func TestRestoreCommand(t *testing.T) {
	tests := []struct {
		db      string
		wantNil bool
	}{
		{"pg-platform", false},
		{"pg-shared", false},
		{"mongo-shared", false},
		{"redis-shared", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		cmd := restoreCommand(tt.db, "/backups/test", testContainerConfig())
		if (cmd == nil) != tt.wantNil {
			t.Errorf("restoreCommand(%q) nil=%v, want nil=%v", tt.db, cmd == nil, tt.wantNil)
		}
	}
}

func TestParseBackupSettings(t *testing.T) {
	m := map[string]string{
		"enabled":        "true",
		"schedule":       "daily",
		"retention_days": "30",
		"databases":      "pg-platform,pg-shared",
	}
	s := ParseBackupSettings(m)
	if !s.Enabled {
		t.Error("expected Enabled=true")
	}
	if s.Schedule != "daily" {
		t.Errorf("Schedule = %q, want daily", s.Schedule)
	}
	if s.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", s.RetentionDays)
	}
	if len(s.Databases) != 2 {
		t.Errorf("Databases len = %d, want 2", len(s.Databases))
	}
}

func TestParseBackupSettingsDefaults(t *testing.T) {
	s := ParseBackupSettings(map[string]string{})
	if s.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if s.Schedule != "daily" {
		t.Errorf("Schedule default = %q, want daily", s.Schedule)
	}
	if s.RetentionDays != 30 {
		t.Errorf("RetentionDays default = %d, want 30", s.RetentionDays)
	}
}

func TestShouldRunNow(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	// daily at 03:00 — exact match
	ts := time.Date(2026, 4, 2, 3, 0, 30, 0, loc)
	if !shouldRunNow("daily", ts, loc) {
		t.Error("daily should run at 03:00")
	}

	// daily at 04:00 — no match
	ts = time.Date(2026, 4, 2, 4, 0, 30, 0, loc)
	if shouldRunNow("daily", ts, loc) {
		t.Error("daily should NOT run at 04:00")
	}

	// weekly on Sunday at 03:00
	// 2026-04-05 is a Sunday
	ts = time.Date(2026, 4, 5, 3, 0, 30, 0, loc)
	if !shouldRunNow("weekly", ts, loc) {
		t.Error("weekly should run on Sunday 03:00")
	}

	// weekly on Monday — no match
	ts = time.Date(2026, 4, 6, 3, 0, 30, 0, loc)
	if shouldRunNow("weekly", ts, loc) {
		t.Error("weekly should NOT run on Monday")
	}

	// monthly on 1st at 03:00
	ts = time.Date(2026, 4, 1, 3, 0, 30, 0, loc)
	if !shouldRunNow("monthly", ts, loc) {
		t.Error("monthly should run on 1st 03:00")
	}

	// monthly on 2nd — no match
	ts = time.Date(2026, 4, 2, 3, 0, 30, 0, loc)
	if shouldRunNow("monthly", ts, loc) {
		t.Error("monthly should NOT run on 2nd")
	}
}

// testContainerConfig returns config for command construction tests.
func testContainerConfig() ContainerConfig {
	return ContainerConfig{
		PGPlatformContainer: "luxview-pg-platform",
		PGPlatformUser:      "luxview",
		PGSharedContainer:   "luxview-pg-shared",
		PGSharedUser:        "luxview_admin",
		MongoContainer:      "luxview-mongo-shared",
		MongoUser:           "luxview_admin",
		MongoPassword:       "testpass",
		RedisContainer:      "luxview-redis-shared",
		RedisPassword:       "testpass",
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd luxview-engine && go test ./internal/service/ -run "TestBuild|TestDump|TestRestore|TestParse|TestShouldRun" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement backup service**

```go
// internal/service/backup_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// ContainerConfig holds container names and credentials needed for backup/restore.
type ContainerConfig struct {
	PGPlatformContainer string
	PGPlatformUser      string
	PGSharedContainer   string
	PGSharedUser        string
	MongoContainer      string
	MongoUser           string
	MongoPassword       string
	RedisContainer      string
	RedisPassword       string
}

type BackupService struct {
	repo         *repository.BackupRepo
	settingsRepo *repository.SettingsRepo
	auditSvc     *AuditService
	backupDir    string
	containers   ContainerConfig
	mu           sync.Mutex
	running      bool
}

func NewBackupService(
	repo *repository.BackupRepo,
	settingsRepo *repository.SettingsRepo,
	auditSvc *AuditService,
	backupDir string,
	containers ContainerConfig,
) *BackupService {
	return &BackupService{
		repo:         repo,
		settingsRepo: settingsRepo,
		auditSvc:     auditSvc,
		backupDir:    backupDir,
		containers:   containers,
	}
}

// IsRunning returns whether a backup or restore is currently in progress.
func (s *BackupService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Run executes a backup for the specified databases.
func (s *BackupService) Run(ctx context.Context, databases []string, trigger model.BackupTrigger, userID *uuid.UUID) (*model.Backup, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, fmt.Errorf("a backup or restore operation is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log := logger.With("backup")
	start := time.Now()

	// Create backup record
	backup := &model.Backup{
		Databases: databases,
		Status:    model.BackupStatusRunning,
		Trigger:   trigger,
		CreatedBy: userID,
	}

	dirName := buildBackupDirName(start, trigger)
	backupPath := filepath.Join(s.backupDir, dirName)
	backup.FilePath = backupPath

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	if err := s.repo.Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("create backup record: %w", err)
	}

	log.Info().Str("id", backup.ID.String()).Strs("databases", databases).Msg("backup started")

	// Execute dumps
	var totalSize int64
	var backupErr error

	for _, db := range databases {
		cmd := dumpCommand(db, backupPath, s.containers)
		if cmd == nil {
			continue
		}

		log.Info().Str("database", db).Msg("backing up database")
		output, err := cmd.CombinedOutput()
		if err != nil {
			backupErr = fmt.Errorf("backup %s failed: %w — %s", db, err, string(output))
			log.Error().Err(backupErr).Str("database", db).Msg("database backup failed")
			break
		}
		log.Info().Str("database", db).Msg("database backup completed")
	}

	// Calculate total size
	totalSize = dirSize(backupPath)
	durationMs := int(time.Since(start).Milliseconds())

	// Write metadata
	meta := map[string]interface{}{
		"databases":   databases,
		"duration_ms": durationMs,
		"file_size":   totalSize,
		"started_at":  start.Format(time.RFC3339),
		"trigger":     string(trigger),
	}
	if metaBytes, err := json.MarshalIndent(meta, "", "  "); err == nil {
		os.WriteFile(filepath.Join(backupPath, "metadata.json"), metaBytes, 0644)
	}

	// Update record
	status := model.BackupStatusCompleted
	errMsg := ""
	if backupErr != nil {
		status = model.BackupStatusFailed
		errMsg = backupErr.Error()
	}

	if err := s.repo.UpdateStatus(ctx, backup.ID, status, errMsg, totalSize, durationMs); err != nil {
		log.Error().Err(err).Msg("failed to update backup status")
	}

	backup.Status = status
	backup.FileSize = totalSize
	backup.DurationMs = durationMs
	backup.Error = errMsg

	log.Info().
		Str("id", backup.ID.String()).
		Str("status", string(status)).
		Int("duration_ms", durationMs).
		Int64("size", totalSize).
		Msg("backup finished")

	return backup, backupErr
}

// Restore restores databases from a backup.
func (s *BackupService) Restore(ctx context.Context, backupID uuid.UUID) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("a backup or restore operation is already running")
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log := logger.With("backup")

	backup, err := s.repo.FindByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup == nil {
		return fmt.Errorf("backup not found")
	}
	if backup.Status != model.BackupStatusCompleted {
		return fmt.Errorf("cannot restore backup with status %s", backup.Status)
	}

	log.Info().Str("id", backupID.String()).Strs("databases", backup.Databases).Msg("restore started")

	// Mark as restoring
	_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusRestoring, "", backup.FileSize, backup.DurationMs)

	for _, db := range backup.Databases {
		cmd := restoreCommand(db, backup.FilePath, s.containers)
		if cmd == nil {
			continue
		}

		log.Info().Str("database", db).Msg("restoring database")
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Revert status
			_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusCompleted, "", backup.FileSize, backup.DurationMs)
			return fmt.Errorf("restore %s failed: %w — %s", db, err, string(output))
		}
		log.Info().Str("database", db).Msg("database restore completed")
	}

	// Restore status back to completed
	_ = s.repo.UpdateStatus(ctx, backupID, model.BackupStatusCompleted, "", backup.FileSize, backup.DurationMs)

	log.Info().Str("id", backupID.String()).Msg("restore finished")
	return nil
}

// Delete removes a backup's files and database record.
func (s *BackupService) Delete(ctx context.Context, backupID uuid.UUID) error {
	backup, err := s.repo.FindByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("find backup: %w", err)
	}
	if backup == nil {
		return fmt.Errorf("backup not found")
	}

	// Remove files
	if backup.FilePath != "" {
		os.RemoveAll(backup.FilePath)
	}

	return s.repo.Delete(ctx, backupID)
}

// Cleanup removes backups older than retentionDays.
func (s *BackupService) Cleanup(ctx context.Context, retentionDays int) error {
	log := logger.With("backup")
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Find old backups to remove files
	backups, _, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return err
	}

	var removed int
	for _, b := range backups {
		if b.StartedAt.Before(cutoff) {
			if b.FilePath != "" {
				os.RemoveAll(b.FilePath)
			}
			removed++
		}
	}

	deleted, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}

	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Int("retention_days", retentionDays).Msg("old backups cleaned up")
	}
	return nil
}

// GetSettings reads backup settings from platform_settings.
func (s *BackupService) GetSettings(ctx context.Context) model.BackupSettings {
	settings, _ := s.settingsRepo.GetAll(ctx, "backup_")
	return ParseBackupSettings(settings)
}

// SaveSettings writes backup settings to platform_settings.
func (s *BackupService) SaveSettings(ctx context.Context, bs model.BackupSettings) error {
	enabled := "false"
	if bs.Enabled {
		enabled = "true"
	}
	if err := s.settingsRepo.Set(ctx, "backup_enabled", enabled, false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_schedule", bs.Schedule, false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_retention_days", strconv.Itoa(bs.RetentionDays), false); err != nil {
		return err
	}
	if err := s.settingsRepo.Set(ctx, "backup_databases", strings.Join(bs.Databases, ","), false); err != nil {
		return err
	}
	return nil
}

// ParseBackupSettings converts a settings map to BackupSettings.
func ParseBackupSettings(m map[string]string) model.BackupSettings {
	s := model.BackupSettings{
		Enabled:       m["enabled"] == "true",
		Schedule:      m["schedule"],
		RetentionDays: 30,
	}
	if s.Schedule == "" {
		s.Schedule = "daily"
	}
	if v, err := strconv.Atoi(m["retention_days"]); err == nil && v > 0 {
		s.RetentionDays = v
	}
	if dbs := m["databases"]; dbs != "" {
		s.Databases = strings.Split(dbs, ",")
	}
	return s
}

// shouldRunNow checks if the schedule matches the current time (within the 03:00 minute).
func shouldRunNow(schedule string, now time.Time, loc *time.Location) bool {
	t := now.In(loc)
	if t.Hour() != 3 || t.Minute() != 0 {
		return false
	}
	switch schedule {
	case "daily":
		return true
	case "weekly":
		return t.Weekday() == time.Sunday
	case "monthly":
		return t.Day() == 1
	}
	return false
}

// buildBackupDirName creates a directory name like "2026-04-02_030000_scheduled".
func buildBackupDirName(t time.Time, trigger model.BackupTrigger) string {
	return fmt.Sprintf("%s_%s", t.Format("2006-01-02_150405"), string(trigger))
}

// dumpCommand returns an exec.Cmd that performs the backup for a given database.
func dumpCommand(db string, backupPath string, cfg ContainerConfig) *exec.Cmd {
	switch db {
	case "pg-platform":
		outFile := filepath.Join(backupPath, "pg-platform.sql.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("docker exec %s pg_dumpall -U %s | gzip > %s",
				cfg.PGPlatformContainer, cfg.PGPlatformUser, outFile))
	case "pg-shared":
		outFile := filepath.Join(backupPath, "pg-shared.sql.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("docker exec %s pg_dumpall -U %s | gzip > %s",
				cfg.PGSharedContainer, cfg.PGSharedUser, outFile))
	case "mongo-shared":
		outFile := filepath.Join(backupPath, "mongo-shared.archive.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("docker exec %s mongodump --authenticationDatabase admin -u %s -p %s --archive --gzip > %s",
				cfg.MongoContainer, cfg.MongoUser, cfg.MongoPassword, outFile))
	case "redis-shared":
		return exec.Command("bash", "-c",
			fmt.Sprintf("docker exec %s redis-cli -a %s BGSAVE && sleep 2 && docker cp %s:/data/dump.rdb %s/redis-shared.rdb",
				cfg.RedisContainer, cfg.RedisPassword, cfg.RedisContainer, backupPath))
	}
	return nil
}

// restoreCommand returns an exec.Cmd that restores a database from backup.
func restoreCommand(db string, backupPath string, cfg ContainerConfig) *exec.Cmd {
	switch db {
	case "pg-platform":
		inFile := filepath.Join(backupPath, "pg-platform.sql.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("gunzip -c %s | docker exec -i %s psql -U %s",
				inFile, cfg.PGPlatformContainer, cfg.PGPlatformUser))
	case "pg-shared":
		inFile := filepath.Join(backupPath, "pg-shared.sql.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("gunzip -c %s | docker exec -i %s psql -U %s",
				inFile, cfg.PGSharedContainer, cfg.PGSharedUser))
	case "mongo-shared":
		inFile := filepath.Join(backupPath, "mongo-shared.archive.gz")
		return exec.Command("bash", "-c",
			fmt.Sprintf("cat %s | docker exec -i %s mongorestore --authenticationDatabase admin -u %s -p %s --archive --gzip --drop",
				inFile, cfg.MongoContainer, cfg.MongoUser, cfg.MongoPassword))
	case "redis-shared":
		rdbFile := filepath.Join(backupPath, "redis-shared.rdb")
		return exec.Command("bash", "-c",
			fmt.Sprintf("docker cp %s %s:/data/dump.rdb && docker restart %s",
				rdbFile, cfg.RedisContainer, cfg.RedisContainer))
	}
	return nil
}

// dirSize returns the total size of all files in a directory.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd luxview-engine && go test ./internal/service/ -run "TestBuild|TestDump|TestRestore|TestParse|TestShouldRun" -v`
Expected: PASS — all tests green

- [ ] **Step 5: Commit**

```bash
git add luxview-engine/internal/service/backup_service.go luxview-engine/internal/service/backup_service_test.go
git commit -m "feat(backup): add backup service with execution, restore, and scheduler logic"
```

---

## Task 5: Backup Worker (Scheduler Goroutine)

**Files:**
- Create: `luxview-engine/internal/worker/backup_worker.go`
- Create: `luxview-engine/internal/worker/backup_worker_test.go`

- [ ] **Step 1: Write failing tests for backup worker**

```go
// internal/worker/backup_worker_test.go
package worker

import "testing"

func TestNewBackupWorker(t *testing.T) {
	w := NewBackupWorker(nil, nil, nil)
	if w == nil {
		t.Fatal("NewBackupWorker returned nil")
	}
	if w.checkInterval != 60 {
		t.Errorf("checkInterval = %d, want 60", w.checkInterval)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd luxview-engine && go test ./internal/worker/ -run TestNewBackupWorker -v`
Expected: FAIL — BackupWorker not defined

- [ ] **Step 3: Implement backup worker**

```go
// internal/worker/backup_worker.go
package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

// BackupWorker periodically checks if a scheduled backup should run.
type BackupWorker struct {
	backupSvc     *service.BackupService
	settingsRepo  *repository.SettingsRepo
	backupRepo    *repository.BackupRepo
	checkInterval int // seconds between checks (60)
	lastRunDate   string
}

func NewBackupWorker(backupSvc *service.BackupService, settingsRepo *repository.SettingsRepo, backupRepo *repository.BackupRepo) *BackupWorker {
	return &BackupWorker{
		backupSvc:     backupSvc,
		settingsRepo:  settingsRepo,
		backupRepo:    backupRepo,
		checkInterval: 60,
	}
}

// Start begins the backup scheduler loop.
func (w *BackupWorker) Start(ctx context.Context) {
	log := logger.With("backup-worker")
	log.Info().Msg("starting backup scheduler")

	ticker := time.NewTicker(time.Duration(w.checkInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("backup scheduler stopped")
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *BackupWorker) check(ctx context.Context) {
	log := logger.With("backup-worker")

	settings := w.backupSvc.GetSettings(ctx)
	if !settings.Enabled || len(settings.Databases) == 0 {
		return
	}

	// Determine timezone
	tz, _ := w.settingsRepo.Get(ctx, "platform_timezone")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	// Prevent running multiple times in the same minute
	dateKey := now.Format("2006-01-02-15-04") + "-" + settings.Schedule
	if dateKey == w.lastRunDate {
		return
	}

	// Check if now matches the schedule
	if now.Hour() != 3 || now.Minute() != 0 {
		return
	}

	shouldRun := false
	switch settings.Schedule {
	case "daily":
		shouldRun = true
	case "weekly":
		shouldRun = now.Weekday() == time.Sunday
	case "monthly":
		shouldRun = now.Day() == 1
	}

	if !shouldRun {
		return
	}

	w.lastRunDate = dateKey
	log.Info().Str("schedule", settings.Schedule).Strs("databases", settings.Databases).Msg("scheduled backup triggered")

	_, err = w.backupSvc.Run(ctx, settings.Databases, model.BackupTriggerScheduled, nil)
	if err != nil {
		log.Error().Err(err).Msg("scheduled backup failed")
	}

	// Cleanup old backups
	if err := w.backupSvc.Cleanup(ctx, settings.RetentionDays); err != nil {
		log.Error().Err(err).Msg("backup cleanup failed")
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd luxview-engine && go test ./internal/worker/ -run TestNewBackupWorker -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add luxview-engine/internal/worker/backup_worker.go luxview-engine/internal/worker/backup_worker_test.go
git commit -m "feat(backup): add backup scheduler worker with tests"
```

---

## Task 6: Backup Handler (HTTP API)

**Files:**
- Create: `luxview-engine/internal/api/handlers/backup_handler.go`
- Create: `luxview-engine/internal/api/handlers/backup_handler_test.go`

- [ ] **Step 1: Write failing tests for backup handler**

```go
// internal/api/handlers/backup_handler_test.go
package handlers

import "testing"

func TestNewBackupHandler(t *testing.T) {
	h := NewBackupHandler(nil, nil)
	if h == nil {
		t.Fatal("NewBackupHandler returned nil")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd luxview-engine && go test ./internal/api/handlers/ -run "TestNewBackupHandler|TestFormatBytes" -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement backup handler**

```go
// internal/api/handlers/backup_handler.go
package handlers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type BackupHandler struct {
	backupSvc *service.BackupService
	auditSvc  *service.AuditService
}

func NewBackupHandler(backupSvc *service.BackupService, auditSvc *service.AuditService) *BackupHandler {
	return &BackupHandler{backupSvc: backupSvc, auditSvc: auditSvc}
}

// List returns paginated backup history.
func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	backups, total, err := h.backupSvc.List(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backups": backups,
		"total":   total,
	})
}

// Get returns a single backup by ID.
func (h *BackupHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	backup, err := h.backupSvc.FindByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get backup")
		return
	}
	if backup == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	writeJSON(w, http.StatusOK, backup)
}

// Trigger starts a manual backup.
func (h *BackupHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	log := logger.With("backup")
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	var req struct {
		Databases []string `json:"databases"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// If no databases specified, use configured ones
	if len(req.Databases) == 0 {
		settings := h.backupSvc.GetSettings(ctx)
		req.Databases = settings.Databases
	}

	if len(req.Databases) == 0 {
		writeError(w, http.StatusBadRequest, "no databases specified")
		return
	}

	// Validate databases
	for _, db := range req.Databases {
		if !model.IsValidDatabase(db) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid database: %s", db))
			return
		}
	}

	if h.backupSvc.IsRunning() {
		writeError(w, http.StatusConflict, "a backup or restore operation is already running")
		return
	}

	userID := user.ID
	go func() {
		backup, err := h.backupSvc.Run(ctx, req.Databases, model.BackupTriggerManual, &userID)
		if err != nil {
			log.Error().Err(err).Msg("manual backup failed")
			return
		}

		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:      user.ID,
			ActorUsername: user.Username,
			Action:       "create",
			ResourceType: "backup",
			ResourceID:   backup.ID.String(),
			ResourceName: "manual backup",
			NewValues:    map[string]interface{}{"databases": req.Databases},
			IPAddress:    clientIP(r),
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"message": "backup started"})
}

// Delete removes a backup.
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	if err := h.backupSvc.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "backup",
		ResourceID:   id.String(),
		ResourceName: "backup",
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "backup deleted"})
}

// Restore restores from a backup with confirmation.
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	log := logger.With("backup")
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Confirm != "RESTORE" {
		writeError(w, http.StatusBadRequest, "confirmation text must be 'RESTORE'")
		return
	}

	if h.backupSvc.IsRunning() {
		writeError(w, http.StatusConflict, "a backup or restore operation is already running")
		return
	}

	go func() {
		err := h.backupSvc.Restore(ctx, id)
		if err != nil {
			log.Error().Err(err).Str("backup_id", id.String()).Msg("restore failed")
			return
		}

		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:      user.ID,
			ActorUsername: user.Username,
			Action:       "restore",
			ResourceType: "backup",
			ResourceID:   id.String(),
			ResourceName: "backup restore",
			IPAddress:    clientIP(r),
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"message": "restore started"})
}

// Download streams the backup directory as a tar.gz archive.
func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	backup, err := h.backupSvc.FindByID(ctx, id)
	if err != nil || backup == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	if backup.Status != model.BackupStatusCompleted {
		writeError(w, http.StatusBadRequest, "backup is not completed")
		return
	}

	// Check directory exists
	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup files not found on disk")
		return
	}

	dirName := filepath.Base(backup.FilePath)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar.gz", dirName))

	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	filepath.Walk(backup.FilePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(backup.FilePath, path)
		header := &tar.Header{
			Name: relPath,
			Size: info.Size(),
			Mode: int64(info.Mode()),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// GetSettings returns backup configuration.
func (h *BackupHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.backupSvc.GetSettings(r.Context())
	writeJSON(w, http.StatusOK, settings)
}

// UpdateSettings updates backup configuration.
func (h *BackupHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	var req model.BackupSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if req.Schedule != "" && !model.IsValidSchedule(req.Schedule) {
		writeError(w, http.StatusBadRequest, "invalid schedule: must be daily, weekly, or monthly")
		return
	}
	if req.RetentionDays != 0 && !model.IsValidRetention(req.RetentionDays) {
		writeError(w, http.StatusBadRequest, "invalid retention: must be 7, 14, 30, or 60")
		return
	}
	for _, db := range req.Databases {
		if !model.IsValidDatabase(db) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid database: %s", db))
			return
		}
	}

	if req.Schedule == "" {
		req.Schedule = "daily"
	}
	if req.RetentionDays == 0 {
		req.RetentionDays = 30
	}

	if err := h.backupSvc.SaveSettings(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "setting",
		ResourceID:   "backup",
		ResourceName: "backup settings",
		NewValues:    map[string]interface{}{"enabled": req.Enabled, "schedule": req.Schedule, "retention_days": req.RetentionDays, "databases": req.Databases},
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "backup settings updated"})
}

// formatBytes converts bytes to human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd luxview-engine && go test ./internal/api/handlers/ -run "TestNewBackupHandler|TestFormatBytes" -v`
Expected: PASS

- [ ] **Step 5: Verify build**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add luxview-engine/internal/api/handlers/backup_handler.go luxview-engine/internal/api/handlers/backup_handler_test.go
git commit -m "feat(backup): add backup HTTP handler with tests"
```

---

## Task 7: Add List and FindByID to BackupService (proxy methods)

The handler calls `h.backupSvc.List()` and `h.backupSvc.FindByID()` but these proxy methods don't exist on BackupService yet.

**Files:**
- Modify: `luxview-engine/internal/service/backup_service.go`

- [ ] **Step 1: Add proxy methods to BackupService**

Append to `backup_service.go`:

```go
// List proxies to the backup repository.
func (s *BackupService) List(ctx context.Context, limit, offset int) ([]model.Backup, int, error) {
	return s.repo.List(ctx, limit, offset)
}

// FindByID proxies to the backup repository.
func (s *BackupService) FindByID(ctx context.Context, id uuid.UUID) (*model.Backup, error) {
	return s.repo.FindByID(ctx, id)
}
```

- [ ] **Step 2: Verify build**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add luxview-engine/internal/service/backup_service.go
git commit -m "feat(backup): add List and FindByID proxy methods to BackupService"
```

---

## Task 8: Wire Everything — Router + Main

**Files:**
- Modify: `luxview-engine/internal/api/router.go`
- Modify: `luxview-engine/cmd/engine/main.go`

- [ ] **Step 1: Add BackupSvc to router Deps and register routes**

In `internal/api/router.go`:

1. Add to `Deps` struct:
```go
	BackupSvc *service.BackupService
```

2. Add import for `service` if not already present (it's used via `service.DeployRequest` already).

3. In `NewRouter()`, after existing handler initialization (e.g., after `mailboxHandler`), add:
```go
	backupHandler := handlers.NewBackupHandler(deps.BackupSvc, deps.AuditSvc)
```

4. In the admin routes group (inside `r.Group(func(r chi.Router) { r.Use(middleware.AdminOnly) ...`), add:
```go
				r.Get("/admin/backups", backupHandler.List)
				r.Post("/admin/backups", backupHandler.Trigger)
				r.Get("/admin/backups/settings", backupHandler.GetSettings)
				r.Put("/admin/backups/settings", backupHandler.UpdateSettings)
				r.Get("/admin/backups/{id}", backupHandler.Get)
				r.Delete("/admin/backups/{id}", backupHandler.Delete)
				r.Post("/admin/backups/{id}/restore", backupHandler.Restore)
				r.Get("/admin/backups/{id}/download", backupHandler.Download)
```

- [ ] **Step 2: Wire services in main.go**

In `cmd/engine/main.go`:

1. After `mailboxRepo` initialization, add:
```go
	backupRepo := repository.NewBackupRepo(db)
```

2. After existing service initialization (e.g., after `alerter`), add:
```go
	backupSvc := service.NewBackupService(backupRepo, settingsRepo, auditSvc, cfg.BackupDir, service.ContainerConfig{
		PGPlatformContainer: "luxview-pg-platform",
		PGPlatformUser:      "luxview",
		PGSharedContainer:   "luxview-pg-shared",
		PGSharedUser:        cfg.SharedPGUser,
		MongoContainer:      "luxview-mongo-shared",
		MongoUser:           cfg.SharedMongoUser,
		MongoPassword:       cfg.SharedMongoPassword,
		RedisContainer:      "luxview-redis-shared",
		RedisPassword:       cfg.SharedRedisPassword,
	})
```

3. After existing worker starts (e.g., after `aggregationWorker`), add:
```go
	backupWorker := worker.NewBackupWorker(backupSvc, settingsRepo, backupRepo)
	go backupWorker.Start(ctx)
```

4. Add `BackupSvc: backupSvc,` to the `api.Deps{}` struct in the router call.

- [ ] **Step 3: Verify build**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add luxview-engine/internal/api/router.go luxview-engine/cmd/engine/main.go
git commit -m "feat(backup): wire backup service, handler, and worker into engine"
```

---

## Task 9: Frontend — API Client + Types

**Files:**
- Create: `luxview-dashboard/src/api/backups.ts`

- [ ] **Step 1: Create backup API client**

```typescript
// src/api/backups.ts
import { api } from './client';

export interface Backup {
  id: string;
  databases: string[];
  status: 'running' | 'completed' | 'failed' | 'restoring';
  trigger: 'scheduled' | 'manual';
  fileSize: number;
  durationMs: number;
  error?: string;
  startedAt: string;
  completedAt?: string;
  createdBy?: string;
  createdAt: string;
}

export interface BackupSettings {
  enabled: boolean;
  schedule: 'daily' | 'weekly' | 'monthly';
  retentionDays: number;
  databases: string[];
}

export const backupsApi = {
  async list(limit = 20, offset = 0): Promise<{ backups: Backup[]; total: number }> {
    const { data } = await api.get<{ backups: Backup[]; total: number }>('/admin/backups', {
      params: { limit, offset },
    });
    return data;
  },

  async get(id: string): Promise<Backup> {
    const { data } = await api.get<Backup>(`/admin/backups/${id}`);
    return data;
  },

  async trigger(databases?: string[]): Promise<void> {
    await api.post('/admin/backups', { databases });
  },

  async remove(id: string): Promise<void> {
    await api.delete(`/admin/backups/${id}`);
  },

  async restore(id: string, confirm: string): Promise<void> {
    await api.post(`/admin/backups/${id}/restore`, { confirm });
  },

  downloadUrl(id: string): string {
    return `/api/admin/backups/${id}/download`;
  },

  async getSettings(): Promise<BackupSettings> {
    const { data } = await api.get<BackupSettings>('/admin/backups/settings');
    return data;
  },

  async updateSettings(settings: BackupSettings): Promise<void> {
    await api.put('/admin/backups/settings', settings);
  },
};
```

- [ ] **Step 2: Commit**

```bash
git add luxview-dashboard/src/api/backups.ts
git commit -m "feat(backup): add frontend API client for backups"
```

---

## Task 10: Frontend — Backups Page

**Files:**
- Create: `luxview-dashboard/src/pages/Backups.tsx`

- [ ] **Step 1: Create the Backups page**

```tsx
// src/pages/Backups.tsx
import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Database,
  Download,
  Trash2,
  RotateCcw,
  Play,
  Save,
  Loader2,
  AlertTriangle,
  CheckCircle2,
  Clock,
  HardDrive,
  Settings2,
  RefreshCw,
} from 'lucide-react';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { backupsApi, type Backup, type BackupSettings } from '../api/backups';

const DB_OPTIONS = [
  { value: 'pg-platform', label: 'PostgreSQL (Platform)' },
  { value: 'pg-shared', label: 'PostgreSQL (Shared)' },
  { value: 'mongo-shared', label: 'MongoDB (Shared)' },
  { value: 'redis-shared', label: 'Redis (Shared)' },
];

const SCHEDULE_OPTIONS = [
  { value: 'daily', labelKey: 'backups.schedule.daily' },
  { value: 'weekly', labelKey: 'backups.schedule.weekly' },
  { value: 'monthly', labelKey: 'backups.schedule.monthly' },
];

const RETENTION_OPTIONS = [
  { value: 7, labelKey: 'backups.retention.7' },
  { value: 14, labelKey: 'backups.retention.14' },
  { value: 30, labelKey: 'backups.retention.30' },
  { value: 60, labelKey: 'backups.retention.60' },
];

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  return `${Math.floor(s / 60)}m ${s % 60}s`;
}

function StatusBadge({ status }: { status: Backup['status'] }) {
  const config = {
    running: { icon: Loader2, color: 'text-blue-400 bg-blue-400/10', animate: 'animate-spin' },
    completed: { icon: CheckCircle2, color: 'text-emerald-400 bg-emerald-400/10', animate: '' },
    failed: { icon: AlertTriangle, color: 'text-red-400 bg-red-400/10', animate: '' },
    restoring: { icon: RotateCcw, color: 'text-amber-400 bg-amber-400/10', animate: 'animate-spin' },
  }[status];

  const Icon = config.icon;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${config.color}`}>
      <Icon size={14} className={config.animate} />
      {status}
    </span>
  );
}

export function Backups() {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [settings, setSettings] = useState<BackupSettings | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [page, setPage] = useState(0);
  const [restoreId, setRestoreId] = useState<string | null>(null);
  const [restoreConfirm, setRestoreConfirm] = useState('');
  const limit = 20;

  const loadData = useCallback(async () => {
    try {
      const [settingsData, backupsData] = await Promise.all([
        backupsApi.getSettings(),
        backupsApi.list(limit, page * limit),
      ]);
      setSettings(settingsData);
      setBackups(backupsData.backups || []);
      setTotal(backupsData.total);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.loadFailed') });
    } finally {
      setLoading(false);
    }
  }, [page, addNotification, t]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Auto-refresh while any backup is running
  useEffect(() => {
    const hasRunning = backups.some((b) => b.status === 'running' || b.status === 'restoring');
    if (!hasRunning) return;
    const interval = setInterval(loadData, 5000);
    return () => clearInterval(interval);
  }, [backups, loadData]);

  const handleSaveSettings = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      await backupsApi.updateSettings(settings);
      addNotification({ type: 'success', title: t('backups.settingsSaved') });
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.saveFailed') });
    } finally {
      setSaving(false);
    }
  };

  const handleTrigger = async () => {
    try {
      await backupsApi.trigger(settings?.databases);
      addNotification({ type: 'info', title: t('backups.backupStarted') });
      setTimeout(loadData, 1000);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.triggerFailed') });
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await backupsApi.remove(id);
      addNotification({ type: 'success', title: t('backups.backupDeleted') });
      loadData();
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.deleteFailed') });
    }
  };

  const handleRestore = async () => {
    if (!restoreId || restoreConfirm !== 'RESTORE') return;
    try {
      await backupsApi.restore(restoreId, restoreConfirm);
      addNotification({ type: 'info', title: t('backups.restoreStarted') });
      setRestoreId(null);
      setRestoreConfirm('');
      setTimeout(loadData, 1000);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.restoreFailed') });
    }
  };

  const cardClass = `rounded-xl border ${isDark ? 'bg-zinc-900/50 border-zinc-800/50' : 'bg-white border-zinc-200'}`;
  const inputClass = `w-full rounded-lg px-3 py-2 text-sm border transition-colors ${isDark ? 'bg-zinc-800 border-zinc-700 text-white' : 'bg-white border-zinc-300 text-zinc-900'} focus:outline-none focus:ring-2 focus:ring-amber-400/50`;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 size={24} className="animate-spin text-amber-400" />
      </div>
    );
  }

  const totalPages = Math.ceil(total / limit);
  const restoreBackup = backups.find((b) => b.id === restoreId);

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-amber-400/10">
            <HardDrive size={24} className="text-amber-400" />
          </div>
          <div>
            <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-white' : 'text-zinc-900'}`}>
              {t('backups.title')}
            </h1>
            <p className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
              {t('backups.subtitle')}
            </p>
          </div>
        </div>
        <button
          onClick={handleTrigger}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-amber-400 text-black font-medium text-sm hover:bg-amber-300 transition-colors active:scale-[0.98]"
        >
          <Play size={16} />
          {t('backups.triggerNow')}
        </button>
      </div>

      {/* Settings Card */}
      <div className={`${cardClass} p-6`}>
        <div className="flex items-center gap-2 mb-4">
          <Settings2 size={18} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
          <h2 className={`text-base font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {t('backups.settings')}
          </h2>
        </div>

        {settings && (
          <div className="space-y-4">
            {/* Enabled toggle */}
            <div className="flex items-center justify-between">
              <label className={`text-sm font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('backups.autoBackup')}
              </label>
              <button
                onClick={() => setSettings({ ...settings, enabled: !settings.enabled })}
                className={`relative w-11 h-6 rounded-full transition-colors ${
                  settings.enabled ? 'bg-amber-400' : isDark ? 'bg-zinc-700' : 'bg-zinc-300'
                }`}
              >
                <span
                  className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${
                    settings.enabled ? 'translate-x-5' : ''
                  }`}
                />
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Schedule */}
              <div>
                <label className={`block text-sm font-medium mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {t('backups.frequency')}
                </label>
                <select
                  value={settings.schedule}
                  onChange={(e) => setSettings({ ...settings, schedule: e.target.value as BackupSettings['schedule'] })}
                  className={inputClass}
                >
                  {SCHEDULE_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
                  ))}
                </select>
              </div>

              {/* Retention */}
              <div>
                <label className={`block text-sm font-medium mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {t('backups.retention')}
                </label>
                <select
                  value={settings.retentionDays}
                  onChange={(e) => setSettings({ ...settings, retentionDays: Number(e.target.value) })}
                  className={inputClass}
                >
                  {RETENTION_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
                  ))}
                </select>
              </div>
            </div>

            {/* Databases */}
            <div>
              <label className={`block text-sm font-medium mb-2 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                {t('backups.databases')}
              </label>
              <div className="flex flex-wrap gap-3">
                {DB_OPTIONS.map((db) => {
                  const checked = settings.databases.includes(db.value);
                  return (
                    <label
                      key={db.value}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                        checked
                          ? 'border-amber-400/50 bg-amber-400/10'
                          : isDark ? 'border-zinc-700 hover:border-zinc-600' : 'border-zinc-200 hover:border-zinc-300'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => {
                          const dbs = checked
                            ? settings.databases.filter((d) => d !== db.value)
                            : [...settings.databases, db.value];
                          setSettings({ ...settings, databases: dbs });
                        }}
                        className="accent-amber-400"
                      />
                      <Database size={14} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
                      <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{db.label}</span>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="flex justify-end">
              <button
                onClick={handleSaveSettings}
                disabled={saving}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-amber-400 text-black font-medium text-sm hover:bg-amber-300 transition-colors disabled:opacity-50 active:scale-[0.98]"
              >
                {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
                {t('common.save')}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Backup History */}
      <div className={cardClass}>
        <div className="flex items-center justify-between p-6 pb-4">
          <h2 className={`text-base font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {t('backups.history')}
          </h2>
          <button onClick={loadData} className={`p-2 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800' : 'hover:bg-zinc-100'}`}>
            <RefreshCw size={16} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
          </button>
        </div>

        {backups.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 px-6">
            <HardDrive size={48} className={isDark ? 'text-zinc-700' : 'text-zinc-300'} />
            <p className={`mt-3 text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              {t('backups.noBackups')}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className={`text-xs uppercase tracking-wider ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                  <th className="px-6 py-3 text-left">{t('backups.table.status')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.databases')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.size')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.duration')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.type')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.date')}</th>
                  <th className="px-6 py-3 text-right">{t('backups.table.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((backup) => (
                  <tr
                    key={backup.id}
                    className={`border-t transition-colors ${isDark ? 'border-zinc-800/50 hover:bg-zinc-800/30' : 'border-zinc-100 hover:bg-zinc-50'}`}
                  >
                    <td className="px-6 py-3">
                      <StatusBadge status={backup.status} />
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex flex-wrap gap-1">
                        {backup.databases.map((db) => (
                          <span
                            key={db}
                            className={`px-2 py-0.5 rounded text-xs ${isDark ? 'bg-zinc-800 text-zinc-400' : 'bg-zinc-100 text-zinc-600'}`}
                          >
                            {db}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {formatBytes(backup.fileSize)}
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {formatDuration(backup.durationMs)}
                    </td>
                    <td className="px-6 py-3">
                      <span className={`inline-flex items-center gap-1 text-xs ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                        {backup.trigger === 'scheduled' ? <Clock size={12} /> : <Play size={12} />}
                        {backup.trigger}
                      </span>
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {new Date(backup.startedAt).toLocaleString()}
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center justify-end gap-1">
                        {backup.status === 'completed' && (
                          <>
                            <a
                              href={backupsApi.downloadUrl(backup.id)}
                              className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-white' : 'hover:bg-zinc-100 text-zinc-500 hover:text-zinc-900'}`}
                              title={t('backups.download')}
                            >
                              <Download size={16} />
                            </a>
                            <button
                              onClick={() => setRestoreId(backup.id)}
                              className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-amber-400' : 'hover:bg-zinc-100 text-zinc-500 hover:text-amber-600'}`}
                              title={t('backups.restore')}
                            >
                              <RotateCcw size={16} />
                            </button>
                          </>
                        )}
                        <button
                          onClick={() => handleDelete(backup.id)}
                          className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-red-400' : 'hover:bg-zinc-100 text-zinc-500 hover:text-red-600'}`}
                          title={t('common.delete')}
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className={`flex items-center justify-between px-6 py-3 border-t ${isDark ? 'border-zinc-800/50' : 'border-zinc-100'}`}>
            <span className={`text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              {t('backups.showing', { from: page * limit + 1, to: Math.min((page + 1) * limit, total), total })}
            </span>
            <div className="flex gap-1">
              <button
                onClick={() => setPage(Math.max(0, page - 1))}
                disabled={page === 0}
                className={`px-3 py-1 rounded text-xs transition-colors disabled:opacity-30 ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('backups.prev')}
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                disabled={page >= totalPages - 1}
                className={`px-3 py-1 rounded text-xs transition-colors disabled:opacity-30 ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('backups.next')}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Restore Dialog */}
      {restoreId && restoreBackup && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className={`${cardClass} p-6 max-w-md w-full mx-4 space-y-4`}>
            <div className="flex items-center gap-2 text-amber-400">
              <AlertTriangle size={20} />
              <h3 className="text-lg font-semibold">{t('backups.restoreTitle')}</h3>
            </div>
            <p className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
              {t('backups.restoreWarning')}
            </p>
            <div className="flex flex-wrap gap-1">
              {restoreBackup.databases.map((db) => (
                <span key={db} className="px-2 py-1 rounded text-xs bg-red-400/10 text-red-400">{db}</span>
              ))}
            </div>
            <div>
              <label className={`block text-sm mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                {t('backups.typeRestore')}
              </label>
              <input
                type="text"
                value={restoreConfirm}
                onChange={(e) => setRestoreConfirm(e.target.value)}
                placeholder="RESTORE"
                className={inputClass}
              />
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => { setRestoreId(null); setRestoreConfirm(''); }}
                className={`px-4 py-2 rounded-lg text-sm ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleRestore}
                disabled={restoreConfirm !== 'RESTORE'}
                className="px-4 py-2 rounded-lg bg-red-500 text-white text-sm font-medium hover:bg-red-600 transition-colors disabled:opacity-30 active:scale-[0.98]"
              >
                {t('backups.confirmRestore')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add luxview-dashboard/src/pages/Backups.tsx
git commit -m "feat(backup): add Backups management page"
```

---

## Task 11: Frontend — Route, Sidebar, Translations

**Files:**
- Modify: `luxview-dashboard/src/App.tsx`
- Modify: `luxview-dashboard/src/components/layout/Sidebar.tsx`
- Modify: `luxview-dashboard/src/i18n/locales/en.json`
- Modify: `luxview-dashboard/src/i18n/locales/pt-BR.json`
- Modify: `luxview-dashboard/src/i18n/locales/es.json`

- [ ] **Step 1: Add route in App.tsx**

Add import at top:
```tsx
import { Backups } from './pages/Backups';
```

Add route inside the dashboard routes (after `admin` line):
```tsx
        <Route path="backups" element={<Backups />} />
```

- [ ] **Step 2: Add sidebar item**

In `src/components/layout/Sidebar.tsx`:

Add `HardDrive` to the Lucide imports:
```tsx
import {
  LayoutDashboard,
  Plus,
  BarChart3,
  Settings,
  Shield,
  HardDrive,
} from 'lucide-react';
```

Add to the `items` array (before the Shield/admin item):
```typescript
  { icon: HardDrive, labelKey: 'layout.sidebar.backups', path: '/dashboard/backups', adminOnly: true },
```

- [ ] **Step 3: Add English translations**

Append to `src/i18n/locales/en.json` (before the closing `}`):

```json
  "layout.sidebar.backups": "Backups",
  "backups.title": "Backups",
  "backups.subtitle": "Manage database backups and restoration",
  "backups.settings": "Configuration",
  "backups.autoBackup": "Automatic backup",
  "backups.frequency": "Frequency",
  "backups.retention": "Retention",
  "backups.databases": "Databases",
  "backups.schedule.daily": "Daily (03:00)",
  "backups.schedule.weekly": "Weekly (Sunday 03:00)",
  "backups.schedule.monthly": "Monthly (1st 03:00)",
  "backups.retention.7": "7 days",
  "backups.retention.14": "14 days",
  "backups.retention.30": "30 days",
  "backups.retention.60": "60 days",
  "backups.history": "Backup History",
  "backups.triggerNow": "Backup now",
  "backups.noBackups": "No backups yet",
  "backups.table.status": "Status",
  "backups.table.databases": "Databases",
  "backups.table.size": "Size",
  "backups.table.duration": "Duration",
  "backups.table.type": "Type",
  "backups.table.date": "Date",
  "backups.table.actions": "Actions",
  "backups.download": "Download",
  "backups.restore": "Restore",
  "backups.restoreTitle": "Restore Backup",
  "backups.restoreWarning": "This will overwrite the current data in the following databases. This action cannot be undone.",
  "backups.typeRestore": "Type RESTORE to confirm",
  "backups.confirmRestore": "Restore",
  "backups.showing": "Showing {{from}}-{{to}} of {{total}}",
  "backups.prev": "Previous",
  "backups.next": "Next",
  "backups.settingsSaved": "Backup settings saved",
  "backups.backupStarted": "Backup started",
  "backups.backupDeleted": "Backup deleted",
  "backups.restoreStarted": "Restore started",
  "backups.errors.loadFailed": "Failed to load backups",
  "backups.errors.saveFailed": "Failed to save settings",
  "backups.errors.triggerFailed": "Failed to start backup",
  "backups.errors.deleteFailed": "Failed to delete backup",
  "backups.errors.restoreFailed": "Failed to start restore"
```

- [ ] **Step 4: Add Portuguese translations**

Append to `src/i18n/locales/pt-BR.json` (before the closing `}`):

```json
  "layout.sidebar.backups": "Backups",
  "backups.title": "Backups",
  "backups.subtitle": "Gerenciar backups e restauracao de bancos de dados",
  "backups.settings": "Configuracao",
  "backups.autoBackup": "Backup automatico",
  "backups.frequency": "Frequencia",
  "backups.retention": "Retencao",
  "backups.databases": "Bancos de dados",
  "backups.schedule.daily": "Diario (03:00)",
  "backups.schedule.weekly": "Semanal (Domingo 03:00)",
  "backups.schedule.monthly": "Mensal (Dia 1 03:00)",
  "backups.retention.7": "7 dias",
  "backups.retention.14": "14 dias",
  "backups.retention.30": "30 dias",
  "backups.retention.60": "60 dias",
  "backups.history": "Historico de Backups",
  "backups.triggerNow": "Backup agora",
  "backups.noBackups": "Nenhum backup ainda",
  "backups.table.status": "Status",
  "backups.table.databases": "Bancos",
  "backups.table.size": "Tamanho",
  "backups.table.duration": "Duracao",
  "backups.table.type": "Tipo",
  "backups.table.date": "Data",
  "backups.table.actions": "Acoes",
  "backups.download": "Download",
  "backups.restore": "Restaurar",
  "backups.restoreTitle": "Restaurar Backup",
  "backups.restoreWarning": "Isso sobrescrevera os dados atuais nos seguintes bancos de dados. Essa acao nao pode ser desfeita.",
  "backups.typeRestore": "Digite RESTORE para confirmar",
  "backups.confirmRestore": "Restaurar",
  "backups.showing": "Mostrando {{from}}-{{to}} de {{total}}",
  "backups.prev": "Anterior",
  "backups.next": "Proximo",
  "backups.settingsSaved": "Configuracoes de backup salvas",
  "backups.backupStarted": "Backup iniciado",
  "backups.backupDeleted": "Backup removido",
  "backups.restoreStarted": "Restauracao iniciada",
  "backups.errors.loadFailed": "Falha ao carregar backups",
  "backups.errors.saveFailed": "Falha ao salvar configuracoes",
  "backups.errors.triggerFailed": "Falha ao iniciar backup",
  "backups.errors.deleteFailed": "Falha ao remover backup",
  "backups.errors.restoreFailed": "Falha ao iniciar restauracao"
```

- [ ] **Step 5: Add Spanish translations**

Append to `src/i18n/locales/es.json` (before the closing `}`):

```json
  "layout.sidebar.backups": "Backups",
  "backups.title": "Backups",
  "backups.subtitle": "Gestionar copias de seguridad y restauracion de bases de datos",
  "backups.settings": "Configuracion",
  "backups.autoBackup": "Backup automatico",
  "backups.frequency": "Frecuencia",
  "backups.retention": "Retencion",
  "backups.databases": "Bases de datos",
  "backups.schedule.daily": "Diario (03:00)",
  "backups.schedule.weekly": "Semanal (Domingo 03:00)",
  "backups.schedule.monthly": "Mensual (Dia 1 03:00)",
  "backups.retention.7": "7 dias",
  "backups.retention.14": "14 dias",
  "backups.retention.30": "30 dias",
  "backups.retention.60": "60 dias",
  "backups.history": "Historial de Backups",
  "backups.triggerNow": "Backup ahora",
  "backups.noBackups": "Sin backups aun",
  "backups.table.status": "Estado",
  "backups.table.databases": "Bases de datos",
  "backups.table.size": "Tamano",
  "backups.table.duration": "Duracion",
  "backups.table.type": "Tipo",
  "backups.table.date": "Fecha",
  "backups.table.actions": "Acciones",
  "backups.download": "Descargar",
  "backups.restore": "Restaurar",
  "backups.restoreTitle": "Restaurar Backup",
  "backups.restoreWarning": "Esto sobrescribira los datos actuales en las siguientes bases de datos. Esta accion no se puede deshacer.",
  "backups.typeRestore": "Escriba RESTORE para confirmar",
  "backups.confirmRestore": "Restaurar",
  "backups.showing": "Mostrando {{from}}-{{to}} de {{total}}",
  "backups.prev": "Anterior",
  "backups.next": "Siguiente",
  "backups.settingsSaved": "Configuracion de backup guardada",
  "backups.backupStarted": "Backup iniciado",
  "backups.backupDeleted": "Backup eliminado",
  "backups.restoreStarted": "Restauracion iniciada",
  "backups.errors.loadFailed": "Error al cargar backups",
  "backups.errors.saveFailed": "Error al guardar configuracion",
  "backups.errors.triggerFailed": "Error al iniciar backup",
  "backups.errors.deleteFailed": "Error al eliminar backup",
  "backups.errors.restoreFailed": "Error al iniciar restauracion"
```

- [ ] **Step 6: Verify frontend build**

Run: `cd luxview-dashboard && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 7: Commit**

```bash
git add luxview-dashboard/src/App.tsx luxview-dashboard/src/components/layout/Sidebar.tsx luxview-dashboard/src/i18n/locales/en.json luxview-dashboard/src/i18n/locales/pt-BR.json luxview-dashboard/src/i18n/locales/es.json
git commit -m "feat(backup): add route, sidebar item, and i18n translations"
```

---

## Task 12: Docker Compose — Mount /backups Volume

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`

- [ ] **Step 1: Add /backups volume to engine service in docker-compose.yml**

In `docker-compose.yml`, add to the `engine` service `volumes`:
```yaml
      - /backups:/backups
```

- [ ] **Step 2: Add /backups volume to engine service in docker-compose.dev.yml**

In `docker-compose.dev.yml`, add to the `engine` service `volumes`:
```yaml
      - ./backups:/backups
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml docker-compose.dev.yml
git commit -m "feat(backup): mount /backups volume in engine container"
```

---

## Task 13: Full Build Verification

- [ ] **Step 1: Run all Go tests**

Run: `cd luxview-engine && go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Build Go binary**

Run: `cd luxview-engine && go build ./...`
Expected: No errors

- [ ] **Step 3: Build frontend**

Run: `cd luxview-dashboard && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 4: Final commit if any fixes needed**

If any fixes were needed during verification, commit them:
```bash
git add -A
git commit -m "fix(backup): address build issues from verification"
```
