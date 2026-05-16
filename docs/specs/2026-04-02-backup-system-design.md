# Database Backup System — Design Spec

**Date:** 2026-04-02
**Status:** Approved

## Overview

A managed backup service integrated into the LuxView engine that allows platform admins to configure, schedule, trigger, restore, and monitor database backups through the API and dashboard UI.

## Requirements

- Admin configures which databases to back up (pg-platform, pg-shared, mongo-shared, redis-shared)
- Preset schedules: daily (03:00), weekly (Sunday 03:00), monthly (1st 03:00) — using platform timezone
- Preset retention: 7, 14, 30, or 60 days with automatic cleanup
- Manual backup trigger via UI
- Full restore from UI with 2-step confirmation (type database name to confirm)
- Download backup files from UI
- Status and history visible in dashboard; sidebar badge on failure
- Local storage (`/backups/`) by default; external storage as future enhancement
- Only one backup or restore operation runs at a time (mutex)

## Data Model

### Table: `backups`

| Column | Type | Description |
|--------|------|-------------|
| id | UUID PK | Unique identifier (default gen_random_uuid) |
| databases | TEXT[] | Databases included in this backup |
| status | TEXT | `running`, `completed`, `failed`, `restoring` |
| trigger | TEXT | `scheduled`, `manual` |
| file_path | TEXT | Directory path on disk |
| file_size | BIGINT | Total size in bytes |
| duration_ms | INT | Execution duration |
| error | TEXT | Error message (null if success) |
| started_at | TIMESTAMPTZ | When execution started |
| completed_at | TIMESTAMPTZ | When execution finished |
| created_by | UUID FK nullable | User who triggered (null if scheduled) |
| created_at | TIMESTAMPTZ | Row creation time |

### Settings in `platform_settings` (existing table)

| Key | Example | Description |
|-----|---------|-------------|
| `backup_enabled` | `true` | Automatic backup toggle |
| `backup_schedule` | `daily` | Preset: `daily`, `weekly`, `monthly` |
| `backup_retention_days` | `30` | Preset: `7`, `14`, `30`, `60` |
| `backup_databases` | `pg-platform,pg-shared` | Selected databases (CSV) |

## Backend Architecture

### Files

```
luxview-engine/
├── migrations/
│   └── 009_backups.sql
├── internal/
│   ├── model/
│   │   └── backup.go              # Backup struct and constants
│   ├── repository/
│   │   └── backup_repo.go         # CRUD for backups table
│   ├── service/
│   │   └── backup_service.go      # Execution, restore, cleanup, scheduler
│   └── api/handlers/
│       └── backup_handler.go      # HTTP endpoints
```

### BackupService

```go
type BackupService struct {
    repo         *repository.BackupRepo
    settingsRepo *repository.SettingsRepo
    auditSvc     *AuditService
    mu           sync.Mutex
    stopCh       chan struct{}
}
```

**Methods:**
- `Run(ctx, databases []string, trigger string, userID *uuid.UUID) (*model.Backup, error)` — Executes backup, writes files, records in DB
- `Restore(ctx, backupID uuid.UUID) error` — Restores from backup directory
- `Delete(ctx, backupID uuid.UUID) error` — Removes files + DB record
- `Cleanup(ctx) error` — Removes expired backups based on retention setting
- `StartScheduler(ctx)` — Goroutine that checks every minute if it's time to run
- `StopScheduler()` — Signals scheduler goroutine to stop
- `ReloadSchedule(ctx)` — Re-reads settings after config change

### Scheduler Logic

Goroutine launched on engine startup:
1. Read `backup_enabled`, `backup_schedule`, `backup_databases` from settings
2. Read `platform_timezone` for time calculations
3. Every 60 seconds, check if current time matches the schedule
4. If match: run `Run()` then `Cleanup()`
5. When settings change via API, call `ReloadSchedule()`

Schedule mapping:
- `daily` → every day at 03:00
- `weekly` → every Sunday at 03:00
- `monthly` → 1st of each month at 03:00

### API Endpoints (admin-only)

| Method | Route | Action |
|--------|-------|--------|
| GET | `/api/backups` | List backups (paginated: limit, offset) |
| POST | `/api/backups` | Trigger manual backup |
| GET | `/api/backups/:id` | Get backup details |
| DELETE | `/api/backups/:id` | Delete backup (file + record) |
| POST | `/api/backups/:id/restore` | Restore backup (body: `{confirm: "database_name"}`) |
| GET | `/api/backups/:id/download` | Download backup as tar.gz stream |
| GET | `/api/backups/settings` | Get backup configuration |
| PUT | `/api/backups/settings` | Update backup configuration |

### Backup Execution

Each database is backed up via `os/exec` calling `docker exec`:

| Database | Command | Output |
|----------|---------|--------|
| pg-platform | `docker exec luxview-pg-platform pg_dumpall -U luxview \| gzip` | `.sql.gz` |
| pg-shared | `docker exec luxview-pg-shared pg_dumpall -U luxview_admin \| gzip` | `.sql.gz` |
| mongo-shared | `docker exec luxview-mongo-shared mongodump --archive --gzip --authenticationDatabase admin` | `.archive.gz` |
| redis-shared | `docker exec luxview-redis-shared redis-cli -a $PASS BGSAVE` + copy RDB | `.rdb` |

### Restore Execution

| Database | Command |
|----------|---------|
| pg-platform | `gunzip \| docker exec -i luxview-pg-platform psql -U luxview` |
| pg-shared | `gunzip \| docker exec -i luxview-pg-shared psql -U luxview_admin` |
| mongo-shared | `docker exec -i luxview-mongo-shared mongorestore --archive --gzip --drop` |
| redis-shared | Copy RDB to volume + `docker restart luxview-redis-shared` |

### File Structure on Disk

```
/backups/
├── 2026-04-02_030000_scheduled/
│   ├── pg-platform.sql.gz
│   ├── pg-shared.sql.gz
│   └── metadata.json
├── 2026-04-02_143022_manual/
│   ├── pg-platform.sql.gz
│   ├── mongo-shared.archive.gz
│   └── metadata.json
```

`metadata.json` contains: `{databases, duration_ms, file_size, started_at, completed_at, trigger}`.

Each backup is a directory (not a single archive) to allow partial restore of individual databases.

### Concurrency

- `sync.Mutex` in BackupService — only 1 operation at a time (backup or restore)
- Manual trigger while backup is running returns HTTP 409 Conflict
- Restore uses the same mutex

### Audit Logging

All operations (create, restore, delete, settings change) are logged via existing `AuditService`.

## Frontend

### Route: `/backups` (admin-only)

**Settings Card (top):**
- Toggle: "Automatic backup" (on/off)
- Select: Schedule (Daily / Weekly / Monthly)
- Select: Retention (7 / 14 / 30 / 60 days)
- Checkboxes: Databases to include
- Save button
- Next scheduled backup indicator

**History Table (below):**
- Columns: Status (badge), Databases (tags), Size, Duration, Type (manual/scheduled), Date, Actions
- Actions: Download, Restore, Delete
- "Create backup now" button in table header
- Pagination
- Empty state when no backups exist

**Restore Dialog (2-step confirmation):**
1. Dialog shows which databases will be overwritten
2. Text input: admin types the primary database name to confirm
3. "Restore" button enabled only when text matches
4. During restore: loading state with status updates

**Sidebar:**
- "Backups" item in admin section with Database/HardDrive icon
- Red badge if last backup failed

### Files

```
luxview-dashboard/src/
├── pages/
│   └── Backups.tsx              # Main backup management page
├── api/
│   └── backups.ts               # API client functions
├── types/
│   └── backup.ts                # TypeScript interfaces
```

## Migration: 009_backups.sql

```sql
CREATE TABLE IF NOT EXISTS backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    databases TEXT[] NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    trigger TEXT NOT NULL,
    file_path TEXT,
    file_size BIGINT DEFAULT 0,
    duration_ms INT DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backups_status ON backups(status);
CREATE INDEX idx_backups_started_at ON backups(started_at DESC);
```

## Out of Scope (Future)

- External storage destinations (S3, GCS, SCP)
- Email notifications on failure
- Per-database restore (restore single DB from a multi-DB backup)
- Backup encryption at rest
- Backup verification (automatic test-restore)
