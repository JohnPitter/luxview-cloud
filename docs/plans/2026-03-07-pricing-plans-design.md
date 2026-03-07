# Pricing Plans — Design Document

**Date:** 2026-03-07
**Status:** Approved

## Objective

Add configurable pricing plans to LuxView Cloud. The admin creates/edits plans from the dashboard, the landing page displays them dynamically, and the system enforces limits per plan (max apps, CPU, memory, services, features).

## Database

New migration `007_create_plans.sql`:

```sql
CREATE TABLE plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL,
  description TEXT,
  price DECIMAL(10,2) NOT NULL DEFAULT 0,
  currency VARCHAR(3) NOT NULL DEFAULT 'USD',
  billing_cycle VARCHAR(20) NOT NULL DEFAULT 'monthly',
  max_apps INT NOT NULL DEFAULT 1,
  max_cpu_per_app DECIMAL(4,2) NOT NULL DEFAULT 0.25,
  max_memory_per_app VARCHAR(10) NOT NULL DEFAULT '512m',
  max_disk_per_app VARCHAR(10) NOT NULL DEFAULT '1g',
  max_services_per_app INT NOT NULL DEFAULT 1,
  auto_deploy_enabled BOOLEAN NOT NULL DEFAULT false,
  custom_domain_enabled BOOLEAN NOT NULL DEFAULT false,
  priority_builds BOOLEAN NOT NULL DEFAULT false,
  highlighted BOOLEAN NOT NULL DEFAULT false,
  sort_order INT NOT NULL DEFAULT 0,
  features JSONB NOT NULL DEFAULT '[]',
  is_active BOOLEAN NOT NULL DEFAULT true,
  is_default BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN plan_id UUID REFERENCES plans(id);
```

- `features`: JSON array of display strings for landing page bullet points.
- `is_default`: Only one plan can be default. Auto-assigned to new users.
- `sort_order`: Controls display order on landing page.

## Backend

### Model (`model/plan.go`)

```go
type Plan struct {
  ID, Name, Description, Currency, BillingCycle string
  Price float64
  MaxApps, MaxServicesPerApp, SortOrder int
  MaxCPUPerApp float64
  MaxMemoryPerApp, MaxDiskPerApp string
  AutoDeployEnabled, CustomDomainEnabled, PriorityBuilds bool
  Highlighted, IsActive, IsDefault bool
  Features []string
  CreatedAt, UpdatedAt time.Time
}
```

### Repository (`repository/plan_repo.go`)

- `Create(plan)`, `Update(plan)`, `Delete(id)` (soft: is_active=false)
- `FindByID(id)`, `FindDefault()`
- `ListAll()` (admin, includes inactive), `ListActive()` (public, sorted by sort_order)
- `SetDefault(id)` — unsets previous default, sets new one

### Handlers (`handlers/plans.go`)

**Admin-only:**
```
POST   /api/admin/plans              — Create plan
GET    /api/admin/plans              — List all plans
PATCH  /api/admin/plans/{id}         — Update plan
DELETE /api/admin/plans/{id}         — Soft delete
PATCH  /api/admin/plans/{id}/default — Set as default
PATCH  /api/admin/users/{id}/plan    — Assign plan to user
```

**Public:**
```
GET    /api/plans                    — List active plans (landing)
```

### Enforcement

In existing handlers:
- `CreateApp`: count user apps vs `plan.MaxApps`
- `CreateService`: count app services vs `plan.MaxServicesPerApp`
- `CreateApp`/`UpdateApp`: validate CPU/memory/disk vs plan limits
- Auto-deploy toggle: check `plan.AutoDeployEnabled`
- Returns 403 with message: `"Plan limit reached: your {plan} plan allows max {limit} {resource}"`

### User Response

`GET /api/auth/me` returns plan info alongside user data.

## Frontend

### Admin Panel — New "Plans" Tab

- **Plan cards grid**: name, price, limits, features, badges (Default, Highlighted, Inactive)
- **Add Plan button**: opens create/edit modal
- **Modal fields**: name, description, price, currency, billing cycle, all limits, feature booleans, features list (dynamic add/remove), sort order, highlighted toggle
- **Card actions**: Edit, Delete (confirm), Set Default, Toggle Highlighted
- **Users tab**: gains "Plan" column with badge + change button

### Landing Page — Dynamic Pricing

- Calls `GET /api/plans` on mount (no auth)
- Responsive grid: 1-3 plans = single row, 4+ = wrapping grid
- Highlighted plan: amber border, "Recommended" badge, `scale-105`
- Card shows: name, price/cycle, features list, boolean feature badges, limit summary, CTA
- States: skeleton loading, error fallback, empty = hide section

### Dashboard Warnings

- Bar at top when user is at 80%+ of any limit
- "New App" button disabled with tooltip when at max apps
- Plan info accessible in auth store (comes from `/api/auth/me`)

## i18n

Static labels use translation keys (`plan.maxApps`, `plan.recommended`, `pricing.perMonth`, etc.). Plan name/description are free text from admin.

## Enforcement UX

- **Block hard**: 403 from API with descriptive message
- **Soft warnings**: dashboard shows usage vs limits when approaching 80%+
- Both combined for good UX
