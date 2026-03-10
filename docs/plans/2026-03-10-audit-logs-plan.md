# Audit Logs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a full audit logging system that records all user actions with value diffs, viewable in the admin panel as table and timeline.

**Architecture:** New `audit_logs` table (BIGSERIAL PK, JSONB diffs), `AuditLogRepo` for CRUD, `AuditService` as fire-and-forget logger injected into all mutating handlers. Admin API with filterable list + stats. Frontend tab with table/timeline toggle.

**Tech Stack:** Go/pgx (backend), React/TypeScript/Tailwind (frontend), PostgreSQL JSONB for diffs.

---

### Task 1: Database Migration

**Files:**
- Modify: `luxview-engine/internal/repository/db.go:178` (append migration to array)

**Step 1:** Add new migration entry before the closing `}` of the migrations slice:

```go
// After line 177: `ALTER TABLE apps ADD COLUMN IF NOT EXISTS custom_dockerfile TEXT DEFAULT NULL`,

`CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_username VARCHAR(100) NOT NULL,
    action VARCHAR(20) NOT NULL,
    resource_type VARCHAR(30) NOT NULL,
    resource_id VARCHAR(100),
    resource_name VARCHAR(200),
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,

`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC)`,
`CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id, created_at DESC)`,
`CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id)`,
```

**Step 2:** Verify: `go build ./...`

**Step 3:** Commit: `feat(db): add audit_logs table migration`

---

### Task 2: AuditLogRepo

**Files:**
- Create: `luxview-engine/internal/repository/audit_repo.go`

**Step 1:** Create repository with these methods:

```go
package repository

// AuditLogRepo — standard repo pattern
// struct: db *DB
// Constructor: NewAuditLogRepo(db *DB)

// AuditLog model (in same file):
type AuditLog struct {
    ID            int64      `json:"id"`
    ActorID       *uuid.UUID `json:"actorId"`
    ActorUsername  string     `json:"actorUsername"`
    Action        string     `json:"action"`
    ResourceType  string     `json:"resourceType"`
    ResourceID    string     `json:"resourceId"`
    ResourceName  string     `json:"resourceName"`
    OldValues     any        `json:"oldValues,omitempty"`
    NewValues     any        `json:"newValues,omitempty"`
    IPAddress     string     `json:"ipAddress"`
    CreatedAt     time.Time  `json:"createdAt"`
}

// AuditFilter for list queries:
type AuditFilter struct {
    ActorID      *uuid.UUID
    Action       string
    ResourceType string
    ResourceID   string
    From         *time.Time
    To           *time.Time
    Search       string
}

// Methods:
// Create(ctx, log *AuditLog) error
//   → INSERT INTO audit_logs (...) VALUES ($1...$10)
//   → ip_address: use pgtype.Inet or cast in SQL: $10::INET
//
// List(ctx, filter AuditFilter, limit, offset int) ([]AuditLog, error)
//   → Build WHERE dynamically based on non-zero filter fields
//   → Search: WHERE (actor_username ILIKE $N OR resource_name ILIKE $N)
//   → ORDER BY created_at DESC, LIMIT, OFFSET
//   → Scan old_values/new_values with pgx json scanning
//
// Count(ctx, filter AuditFilter) (int64, error)
//   → Same WHERE as List, SELECT COUNT(*)
//
// DeleteOlderThan(ctx, cutoff time.Time) (int64, error)
//   → DELETE FROM audit_logs WHERE created_at < $1
//   → Return rows affected
```

**Step 2:** Verify: `go build ./...`

**Step 3:** Commit: `feat(repo): add AuditLogRepo`

---

### Task 3: AuditService

**Files:**
- Create: `luxview-engine/internal/service/audit.go`

**Step 1:** Create service:

```go
package service

// AuditService wraps AuditLogRepo with fire-and-forget logging.
// struct: repo *repository.AuditLogRepo
// Constructor: NewAuditService(repo *repository.AuditLogRepo)

// AuditEntry — input struct (same fields as AuditLog minus ID/CreatedAt)
type AuditEntry struct {
    ActorID       uuid.UUID
    ActorUsername  string
    Action        string
    ResourceType  string
    ResourceID    string
    ResourceName  string
    OldValues     any
    NewValues     any
    IPAddress     string
}

// Log(ctx, entry AuditEntry) — fire and forget
//   → go func() { repo.Create(bg context, &AuditLog{...}) }()
//   → Use context.Background() for the goroutine (parent ctx may cancel)
//   → Log errors via logger.With("audit").Error() but never propagate
```

**Step 2:** Verify: `go build ./...`

**Step 3:** Commit: `feat(service): add AuditService`

---

### Task 4: Audit Handler + Routes

**Files:**
- Create: `luxview-engine/internal/api/handlers/audit_handler.go`
- Modify: `luxview-engine/internal/api/router.go`
- Modify: `luxview-engine/cmd/engine/main.go`

**Step 1:** Create handler:

```go
// AuditHandler struct: auditRepo *repository.AuditLogRepo
// Constructor: NewAuditHandler(auditRepo *repository.AuditLogRepo)

// ListAuditLogs(w, r) — GET /admin/audit-logs
//   → Parse query params: limit, offset, actor_id, action, resource_type, resource_id, from, to, search
//   → Call repo.List + repo.Count
//   → Return { "logs": [...], "total": N }

// AuditStats(w, r) — GET /admin/audit-logs/stats
//   → Query: SELECT action, COUNT(*) FROM audit_logs WHERE created_at > NOW() - INTERVAL '24 hours' GROUP BY action
//   → Query: SELECT resource_type, COUNT(*) FROM audit_logs WHERE created_at > NOW() - INTERVAL '24 hours' GROUP BY resource_type
//   → Return { "total24h": N, "byAction": {...}, "byResource": {...} }
```

**Step 2:** Add to router.go:
- Add `AuditRepo *repository.AuditLogRepo` and `AuditSvc *service.AuditService` to Deps struct
- Initialize: `auditHandler := handlers.NewAuditHandler(deps.AuditRepo)`
- Add admin routes:
  ```go
  r.Get("/admin/audit-logs", auditHandler.ListAuditLogs)
  r.Get("/admin/audit-logs/stats", auditHandler.AuditStats)
  ```

**Step 3:** Update main.go:
- Create: `auditRepo := repository.NewAuditLogRepo(db)`
- Create: `auditSvc := service.NewAuditService(auditRepo)`
- Add to Deps: `AuditRepo: auditRepo, AuditSvc: auditSvc`

**Step 4:** Verify: `go build ./...`

**Step 5:** Commit: `feat(api): add audit log endpoints`

---

### Task 5: Inject AuditService into Existing Handlers

**Files to modify** (add `auditSvc *service.AuditService` field + constructor param):
- `handlers/apps.go` — AppHandler
- `handlers/admin.go` — AdminHandler
- `handlers/services.go` — ServiceHandler
- `handlers/plans.go` — PlanHandler
- `handlers/deployments.go` — DeploymentHandler
- `handlers/settings_handler.go` — SettingsHandler
- `handlers/cleanup_handler.go` — CleanupHandler
- `handlers/alerts.go` — AlertHandler
- `handlers/auth.go` — AuthHandler
- `handlers/analyze_handler.go` — AnalyzeHandler (for apply-analysis)

**For each handler:**

1. Add `auditSvc *service.AuditService` to struct
2. Add param to constructor, update router.go to pass `deps.AuditSvc`
3. Add `auditSvc.Log()` call after each successful mutation:

**Audit points per handler:**

| Handler | Method | Action | ResourceType | Values to log |
|---------|--------|--------|-------------|---------------|
| AppHandler | Create | create | app | new: {name, subdomain, repoUrl} |
| AppHandler | Update | update | app | old/new: changed fields |
| AppHandler | Delete | delete | app | old: {name, subdomain} |
| AppHandler | Deploy | deploy | app | new: {branch} |
| AppHandler | Restart | restart | app | — |
| AppHandler | Stop | stop | app | — |
| AdminHandler | ForceDeleteApp | delete | app | old: {name, subdomain} |
| AdminHandler | UpdateUserRole | update | user | old/new: {role} |
| AdminHandler | UpdateAppLimits | update | app | old/new: {cpu, memory, disk} |
| ServiceHandler | Create | create | service | new: {type, name} |
| ServiceHandler | Delete | delete | service | old: {type, name} |
| PlanHandler | Create | create | plan | new: {name, price} |
| PlanHandler | Update | update | plan | old/new: changed fields |
| PlanHandler | Delete | delete | plan | old: {name} |
| PlanHandler | SetDefault | update | plan | new: {isDefault: true} |
| PlanHandler | AssignUserPlan | update | user | old/new: {planId} |
| DeploymentHandler | Rollback | deploy | deployment | new: {targetDeployId} |
| SettingsHandler | UpdateAI | update | setting | new: {aiEnabled, aiModel} (mask apiKey) |
| CleanupHandler | UpdateSettings | update | setting | new: {enabled, threshold} |
| CleanupHandler | TriggerCleanup | create | cleanup | new: {manual: true} |
| AlertHandler | Create | create | alert | new: {metric, threshold} |
| AlertHandler | Update | update | alert | old/new: changed fields |
| AlertHandler | Delete | delete | alert | old: {name} |
| AuthHandler | GitHubCallback | login | user | new: {username} |
| AnalyzeHandler | ApplyAnalysis | create | deployment | new: {services} |

**IP extraction pattern** (use in all handlers):
```go
ip := r.Header.Get("X-Forwarded-For")
if ip == "" { ip = r.RemoteAddr }
```

**Actor extraction pattern:**
```go
user := middleware.GetUser(ctx)
// actorID = user.ID, actorUsername = user.Username
```

**Step:** After all handlers updated, verify: `go build ./...`

**Commit:** `feat(audit): instrument all handlers with audit logging`

---

### Task 6: Cleanup Worker Integration

**Files:**
- Modify: `luxview-engine/internal/worker/cleanup_worker.go`

**Step 1:** Add `auditRepo *repository.AuditLogRepo` to CleanupWorker struct + constructor.

**Step 2:** In `cleanup()` method, after metrics cleanup, add:
```go
// Delete audit logs older than 90 days
auditCutoff := time.Now().Add(-90 * 24 * time.Hour)
auditDeleted, err := cw.auditRepo.DeleteOlderThan(ctx, auditCutoff)
if err != nil {
    log.Error().Err(err).Msg("failed to cleanup old audit logs")
} else if auditDeleted > 0 {
    log.Info().Int64("deleted", auditDeleted).Msg("old audit logs cleaned up")
}
```

**Step 3:** Update main.go to pass `auditRepo` to `NewCleanupWorker`.

**Step 4:** Verify: `go build ./...`

**Step 5:** Commit: `feat(worker): add audit log cleanup (90 days retention)`

---

### Task 7: Frontend API Client

**Files:**
- Modify: `luxview-dashboard/src/api/admin.ts`

**Step 1:** Add types and api methods:

```typescript
export interface AuditLog {
  id: number;
  actorId: string | null;
  actorUsername: string;
  action: 'create' | 'update' | 'delete' | 'deploy' | 'restart' | 'stop' | 'login';
  resourceType: string;
  resourceId: string;
  resourceName: string;
  oldValues?: Record<string, unknown>;
  newValues?: Record<string, unknown>;
  ipAddress: string;
  createdAt: string;
}

export interface AuditStats {
  total24h: number;
  byAction: Record<string, number>;
  byResource: Record<string, number>;
}

export interface AuditLogFilters {
  actorId?: string;
  action?: string;
  resourceType?: string;
  search?: string;
  from?: string;
  to?: string;
}

export const auditApi = {
  async list(filters: AuditLogFilters = {}, limit = 50, offset = 0): Promise<{ logs: AuditLog[]; total: number }> {
    const { data } = await api.get('/admin/audit-logs', {
      params: { ...filters, limit, offset },
    });
    return data;
  },
  async stats(): Promise<AuditStats> {
    const { data } = await api.get('/admin/audit-logs/stats');
    return data;
  },
};
```

**Step 2:** Verify: `npx tsc --noEmit`

**Step 3:** Commit: `feat(api): add audit log API client`

---

### Task 8: Frontend Audit Tab — Table View

**Files:**
- Modify: `luxview-dashboard/src/pages/Admin.tsx`

**Step 1:** Add `'audit'` to Tab type, add tab entry with `FileText` icon from Lucide.

**Step 2:** Add state: auditLogs, auditStats, auditLoading, auditFilters, auditPage, auditView ('table'|'timeline'), auditTotal.

**Step 3:** Add fetchAuditLogs callback, useEffect on tab === 'audit', auto-refresh 30s.

**Step 4:** Add table view section:
- Filter bar: action select, resource_type select, search input, date inputs
- Table with columns: Time (relative), User (avatar placeholder + username), Action (colored badge), Resource (type badge + name), Details (expand button)
- Expanded row: diff view with old (red strikethrough) → new (green) values
- Pagination with page numbers

**Badge colors:**
- create: emerald
- update: blue
- delete: red
- deploy/restart/stop: purple
- login: zinc

**Step 5:** Verify: `npx tsc --noEmit`

**Step 6:** Commit: `feat(ui): add audit log table view`

---

### Task 9: Frontend Audit Tab — Timeline View

**Files:**
- Modify: `luxview-dashboard/src/pages/Admin.tsx`

**Step 1:** Add timeline view with toggle buttons (List icon / Activity icon) in header.

**Step 2:** Timeline structure:
- Group logs by day ("Today", "Yesterday", date)
- Vertical line connector (left border)
- Each entry: colored dot + icon by resourceType + "**username** action **resourceName**" text
- Hover/click expands to show diff + IP + exact timestamp

**Resource icons:**
- app: Server
- user: Users
- service: Database
- plan: CreditCard
- setting: SlidersHorizontal
- deployment: Rocket
- alert: Activity
- cleanup: Trash2

**Step 3:** Add badge in tab header showing stats.total24h.

**Step 4:** Verify: `npx tsc --noEmit`

**Step 5:** Commit: `feat(ui): add audit log timeline view`

---

### Task 10: i18n Keys

**Files:**
- Modify: `luxview-dashboard/src/i18n/locales/en.json`
- Modify: `luxview-dashboard/src/i18n/locales/pt-BR.json`
- Modify: `luxview-dashboard/src/i18n/locales/es.json`

**Keys needed:**
```
admin.tabs.audit
admin.audit.title
admin.audit.description
admin.audit.noLogs
admin.audit.search
admin.audit.allActions
admin.audit.allResources
admin.audit.from
admin.audit.to
admin.audit.viewTable
admin.audit.viewTimeline
admin.audit.today
admin.audit.yesterday
admin.audit.expandDetails
admin.audit.ipAddress
admin.audit.oldValue
admin.audit.newValue
admin.audit.last24h
admin.audit.actions.[create|update|delete|deploy|restart|stop|login]
admin.audit.resources.[app|user|service|plan|setting|deployment|alert|cleanup]
```

**Commit:** `feat(i18n): add audit log translations`

---

### Task 11: Build Verification + Deploy

**Step 1:** `go build ./...` — must pass
**Step 2:** `npx tsc --noEmit` — must pass
**Step 3:** Commit any remaining fixes
**Step 4:** Push to GitHub + deploy to VPS (same pattern as before: git pull, docker compose build, up --force-recreate)
