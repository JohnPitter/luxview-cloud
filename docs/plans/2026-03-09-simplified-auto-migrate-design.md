# Simplified Auto-Migrate → Analyze & Deploy

**Date**: 2026-03-09
**Status**: Approved
**Scope**: Replace auto-migrate with lightweight analyze + deploy flow

---

## 1. Overview

Simplify the auto-migrate agent to **only analyze the repository** and produce:
- An optimized Dockerfile
- A list of required environment variables
- Recommended managed services (PostgreSQL, Redis, etc.)

**Removed**: All code modification, PR creation, branch management, and automatic code migration.

**Flow**: `analyze → user approves/edits → apply (save Dockerfile + env vars + provision services) → deploy`

---

## 2. Backend — New `/analyze` Endpoint

**`POST /api/apps/{id}/analyze`** (replaces `/auto-migrate`):

1. Receives: `repoUrl`, `branch` (optional, default `main`)
2. Shallow clones the repo into a temp directory
3. Runs deterministic detection (no AI required):
   - **Runtime**: `package.json` → Node, `requirements.txt`/`pyproject.toml` → Python, `go.mod` → Go, `Gemfile` → Ruby, `pom.xml`/`build.gradle` → Java
   - **Framework**: Next.js, Vite, Express, FastAPI, Django, Gin, NestJS, Flask, etc.
   - **Default port** based on framework
   - **Services**: `prisma/schema.prisma` with `postgresql` → PostgreSQL; `redis`/`ioredis` in deps → Redis; etc.
4. **If AI enabled**: Sends key files (existing Dockerfile, package.json, config files) to LLM with prompt focused **exclusively** on:
   - Generate optimized Dockerfile (multi-stage, alpine, .dockerignore)
   - List required env vars (with placeholder values and descriptions)
   - Confirm/refine service detection
5. **If AI disabled**: Uses pre-defined Dockerfile templates per runtime/framework + env var list from deterministic detection
6. **Returns**:
```json
{
  "dockerfile": "string",
  "envVars": [{"key": "string", "description": "string", "required": true, "defaultValue": "string"}],
  "services": [{"type": "postgresql|redis|mysql|mongodb", "reason": "string"}],
  "detectedRuntime": "string",
  "detectedFramework": "string"
}
```

**No side effects** — doesn't modify anything, doesn't provision anything, just returns the analysis.

### Apply Endpoint

**`POST /api/apps/{id}/apply-analysis`** — called after user approves:
- Saves Dockerfile to app config
- Creates env vars
- Provisions selected services (creates DB/Redis containers via engine)
- Injects generated env vars (e.g., `DATABASE_URL`) into the app
- Triggers deploy

---

## 3. Frontend — Setup Wizard Changes

### AI Toggle Respect

When admin disables AI generation (`settings.aiGeneration === false`), the analysis step **does not render** in the wizard. The wizard skips directly from basic info to manual configuration.

### New Wizard Flow

1. **Basic info** — name, repo URL, branch (unchanged)
2. **Repository analysis** (only if AI enabled):
   - Calls `POST /api/apps/{id}/analyze`
   - Shows results in organized cards:
     - **Generated Dockerfile** — preview with syntax highlighting, "Approve" or "Edit" button
     - **Detected env vars** — editable table (key, value, description, required badge)
     - **Recommended services** — cards with toggle (e.g., "PostgreSQL — detected in prisma schema")
   - Buttons: "Approve and continue" | "Skip (configure manually)"
3. **Manual configuration** — if skipped step 2, or if AI disabled:
   - Upload/edit Dockerfile (or use default template)
   - Manual env vars form
   - Manual service selection
4. **Review & Deploy** — summary → confirm → apply-analysis → deploy

### UI Changes to `DeployAnalysis.tsx`

**Removed**: auto mode, code generation progress, PR preview, migration steps
**New**: Dockerfile preview + editable env vars table + service toggle cards

---

## 4. Deterministic Detection (AI-off Fallback)

### Dockerfile Templates by Runtime

- **Node.js**: Detects Next.js/Vite/Express/NestJS → multi-stage `node:20-alpine`
- **Python**: Detects Django/FastAPI/Flask → `python:3.12-slim` + gunicorn/uvicorn
- **Go**: `golang:1.22-alpine` build + `scratch`/`alpine` runtime
- **Ruby**: `ruby:3.3-slim` + bundler
- **Java**: Maven/Gradle build stage + JRE runtime

### Env Var Detection by Convention

- Scan `.env.example`, `.env.sample`, `.env.template` → extract keys
- Scan `docker-compose.yml` → extract `environment:` section
- Code patterns: `process.env.XXX`, `os.environ["XXX"]`, `os.Getenv("XXX")`

### Service Detection

- **PostgreSQL**: prisma with `postgresql`, `pg` in dependencies, `DATABASE_URL` referenced
- **MySQL**: `mysql2` in dependencies, prisma with `mysql`
- **Redis**: `redis`/`ioredis` in dependencies, `REDIS_URL` referenced
- **MongoDB**: `mongoose`/`mongodb` in dependencies

---

## 5. What to Remove / Modify

### Engine (Go)

| File | Action |
|------|--------|
| `internal/api/handlers/auto_migrate.go` | **Delete** |
| `internal/api/handlers/analyze.go` | **Create** — new `/analyze` + `/apply-analysis` handlers |
| `internal/agent/prompt.go` | Remove `migrationSystemPrompt`, add `analyzeSystemPrompt` |
| `internal/api/router.go` | Replace `auto-migrate` route with `analyze` + `apply-analysis` |
| `internal/api/handlers/analyze_failure.go` | **Keep as-is** |
| `internal/detector/` | **Create** — deterministic detection package |
| `internal/detector/templates/` | **Create** — Dockerfile templates per runtime |

### Dashboard (React)

| File | Action |
|------|--------|
| `src/api/analyze.ts` | Rewrite — `analyzeApi.analyze()` + `analyzeApi.applyAnalysis()`, remove `autoMigrate()` |
| `src/components/deploy/DeployAnalysis.tsx` | Rewrite — Dockerfile preview + editable env vars + service toggles |
| Setup wizard component | Condition analysis step on `settings.aiGeneration` |
| `src/i18n/locales/*.json` | Update translation keys |

### What Disappears from UX

- "Auto Migrate" button
- Migration progress (analyzing → generating → validating → creating PR)
- Generated code preview / diff
- PR link

### What Appears in UX

- Dockerfile preview with syntax highlight
- Editable env vars table
- Service cards with on/off toggle
- Linear flow: analyze → approve → provision → deploy

---

## 6. analyze-failure

**Kept as-is.** Receives build logs, diagnoses the error, suggests Dockerfile fix + env var changes. Prompt `buildFixSystemPrompt` remains unchanged.
