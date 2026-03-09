# Implementation Plan: Simplified Auto-Migrate

**Date**: 2026-03-09
**Design doc**: `docs/plans/2026-03-09-simplified-auto-migrate-design.md`
**Complexity**: Complex (multi-file, backend + frontend, new package)

---

## Objective

Replace the auto-migrate flow (code generation + PR creation) with a lightweight analyze + approve + provision + deploy flow. When AI is disabled, use deterministic detection with Dockerfile templates.

---

## Implementation Order

### Phase A: Backend — Remove auto-migrate, add apply-analysis endpoint
### Phase B: Backend — Add deterministic detector package
### Phase C: Backend — Update analyze endpoint to use detector as fallback
### Phase D: Frontend — Simplify ServiceRecommendationCard
### Phase E: Frontend — Simplify DeployAnalysis + DeployWizard + NewApp
### Phase F: Frontend — Respect AI toggle in wizard

---

## Phase A: Backend — Remove auto-migrate, add apply-analysis

### Step 1: Delete `auto_migrate.go`

**File**: `luxview-engine/internal/api/handlers/auto_migrate.go`
**Action**: Delete entire file

### Step 2: Remove `migrationSystemPrompt` and `buildFixSystemPrompt` from `prompt.go`

**File**: `luxview-engine/internal/agent/prompt.go`
**Action**: Modify — delete lines 178-306 (`migrationSystemPrompt`, `buildFixSystemPrompt`)
Also delete `BuildMigrationContext` (lines 339-370), `readAllSourceFiles` (lines 548-603), `isSkippableForMigration` (lines 605-627), `sourceCodeExtensions` (lines 73-79), `skipSourceDirs` (lines 82-89), `maxMigrationContext` (line 15)

### Step 3: Remove `GenerateCodeChanges`, `FixBuildErrors` from `deploy_agent.go`

**File**: `luxview-engine/internal/agent/deploy_agent.go`
**Action**: Modify — delete:
- `MigrationResult` struct (lines 80-84)
- `GenerateCodeChanges` method (lines 86-127)
- `BuildFixResult` struct (lines 129-132)
- `FixBuildErrors` method (lines 134-173)

### Step 4: Remove `CodeChange` from `types.go`

**File**: `luxview-engine/internal/agent/types.go`
**Action**: Modify — remove `CodeChange` struct (lines 39-44) and `CodeChanges` field from `ServiceRecommendation` (line 35)

```go
// ServiceRecommendation — after change:
type ServiceRecommendation struct {
	CurrentService     string   `json:"currentService"`
	CurrentEvidence    string   `json:"currentEvidence"`
	RecommendedService string   `json:"recommendedService"`
	Reason             string   `json:"reason"`
	ManualSteps        []string `json:"manualSteps"`
}
```

### Step 5: Remove auto-migrate route, add apply-analysis route in `router.go`

**File**: `luxview-engine/internal/api/router.go`
**Action**: Modify

Remove:
```go
autoMigrateHandler := handlers.NewAutoMigrateHandler(deps.AppRepo, deps.UserRepo, deps.ServiceRepo, deps.SettingsRepo, deps.Provisioner, deps.EncryptKey)
```
```go
r.Post("/apps/{id}/auto-migrate", autoMigrateHandler.AutoMigrate)
```

Add to `AnalyzeHandler` constructor (needs ServiceRepo + Provisioner):
```go
analyzeHandler := handlers.NewAnalyzeHandler(deps.AppRepo, deps.UserRepo, deps.DeployRepo, deps.SettingsRepo, deps.ServiceRepo, deps.Provisioner, deps.EncryptKey)
```

Add route:
```go
r.Post("/apps/{id}/apply-analysis", analyzeHandler.ApplyAnalysis)
```

### Step 6: Add `ApplyAnalysis` handler to `analyze_handler.go`

**File**: `luxview-engine/internal/api/handlers/analyze_handler.go`
**Action**: Modify — add ServiceRepo + Provisioner fields, add `ApplyAnalysis` method

Update struct:
```go
type AnalyzeHandler struct {
	appRepo      *repository.AppRepo
	userRepo     *repository.UserRepo
	deployRepo   *repository.DeploymentRepo
	settingsRepo *repository.SettingsRepo
	serviceRepo  *repository.ServiceRepo
	provisioner  *service.Provisioner
	agent        *agent.DeployAgent
	encryptKey   []byte
}
```

Update constructor to accept `serviceRepo *repository.ServiceRepo, provisioner *service.Provisioner`.

Add imports: `"github.com/luxview/engine/internal/model"`, `"github.com/luxview/engine/internal/service"`

New handler:
```go
type applyAnalysisRequest struct {
	Dockerfile string            `json:"dockerfile"`
	EnvVars    map[string]string `json:"envVars"`
	Services   []string          `json:"services"` // ["postgres", "redis"]
}

type applyAnalysisResponse struct {
	Message          string            `json:"message"`
	ProvisionedEnvs  map[string]string `json:"provisionedEnvs,omitempty"`
}

// ApplyAnalysis handles POST /apps/{id}/apply-analysis
// Saves Dockerfile, provisions selected services, injects env vars.
func (h *AnalyzeHandler) ApplyAnalysis(w http.ResponseWriter, r *http.Request) {
	log := logger.With("analyze")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req applyAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Save Dockerfile
	if req.Dockerfile != "" {
		if err := h.appRepo.UpdateCustomDockerfile(ctx, appID, &req.Dockerfile); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save dockerfile")
			return
		}
		log.Info().Str("app", app.Subdomain).Msg("dockerfile saved via apply-analysis")
	}

	// Provision services
	provisionedEnvs := make(map[string]string)
	for _, svcType := range req.Services {
		serviceType := model.ServiceType(svcType)
		svc, err := h.provisioner.Provision(ctx, appID, serviceType)
		if err != nil {
			if strings.Contains(err.Error(), "already provisioned") {
				existing, findErr := h.serviceRepo.FindByAppAndType(ctx, appID, serviceType)
				if findErr != nil || existing == nil {
					log.Warn().Str("service", svcType).Err(err).Msg("failed to find existing service")
					continue
				}
				svc = existing
			} else {
				log.Error().Str("service", svcType).Err(err).Msg("failed to provision service")
				continue
			}
		}
		log.Info().Str("service", svcType).Str("id", svc.ID.String()).Msg("service provisioned via apply-analysis")
		// The provisioner auto-injects env vars into the app — no extra action needed
	}

	// Save user-provided env vars
	if len(req.EnvVars) > 0 {
		// Merge with existing env vars
		existingEnvs := app.EnvVars
		if existingEnvs == nil {
			existingEnvs = make(map[string]string)
		}
		for k, v := range req.EnvVars {
			if v != "" {
				existingEnvs[k] = v
			}
		}
		if err := h.appRepo.UpdateEnvVars(ctx, appID, existingEnvs); err != nil {
			log.Warn().Err(err).Msg("failed to update env vars")
		}
	}

	writeJSON(w, http.StatusOK, applyAnalysisResponse{
		Message:         "Analysis applied successfully",
		ProvisionedEnvs: provisionedEnvs,
	})
}
```

### Step 7: Update `Analyze` handler to work without AI

**File**: `luxview-engine/internal/api/handlers/analyze_handler.go`
**Action**: Modify the `Analyze` method

Currently, if AI is disabled, it returns an error. Change it to:
- If AI enabled → call LLM (existing flow)
- If AI disabled → call deterministic detector (Phase B) → return result

```go
func (h *AnalyzeHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	// ... existing auth/app checks ...

	cfg, err := h.getAIConfig(ctx)

	lang := r.Header.Get("Accept-Language")
	if lang == "" {
		lang = "en"
	}

	cloneDir, err2 := h.cloneRepo(ctx, appID, app.RepoURL, app.RepoBranch)
	if err2 != nil {
		log.Error().Err(err2).Msg("failed to clone repo")
		writeError(w, http.StatusInternalServerError, "failed to clone repository")
		return
	}
	defer os.RemoveAll(cloneDir)

	if err != nil {
		// AI disabled — use deterministic detection
		log.Info().Msg("AI unavailable, using deterministic analysis")
		result := detector.Analyze(cloneDir)
		writeJSON(w, http.StatusOK, result)
		return
	}

	// AI enabled — existing flow
	result, err := h.agent.Analyze(ctx, cfg.apiKey, cfg.model, cloneDir, lang)
	if err != nil {
		log.Error().Err(err).Str("app", app.Subdomain).Msg("analysis failed")
		writeError(w, http.StatusInternalServerError, "analysis failed: "+err.Error())
		return
	}

	log.Info().Str("app", app.Subdomain).Str("stack", result.Stack).Msg("analysis complete")
	writeJSON(w, http.StatusOK, result)
}
```

---

## Phase B: Backend — Deterministic Detector Package

### Step 1: Create `internal/detector/detector.go`

**File**: `luxview-engine/internal/detector/detector.go`
**Action**: Create

```go
package detector

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

type Detection struct {
	Runtime   string
	Framework string
	Port      int
}

// Analyze performs deterministic analysis of a repository.
func Analyze(repoDir string) *agent.AnalysisResult {
	det := detect(repoDir)
	envVars := detectEnvVars(repoDir)
	services := detectServices(repoDir)
	dockerfile := generateDockerfile(det, repoDir)

	return &agent.AnalysisResult{
		Suggestions:            []agent.Suggestion{},
		Dockerfile:             dockerfile,
		Port:                   det.Port,
		Stack:                  det.Runtime,
		EnvHints:               envVars,
		ServiceRecommendations: services,
	}
}

func detect(repoDir string) Detection {
	// Node.js
	if fileExists(repoDir, "package.json") {
		pkg := readFile(repoDir, "package.json")
		if strings.Contains(pkg, "\"next\"") {
			return Detection{Runtime: "nodejs", Framework: "nextjs", Port: 3000}
		}
		if strings.Contains(pkg, "\"vite\"") || strings.Contains(pkg, "\"@vitejs/") {
			return Detection{Runtime: "nodejs", Framework: "vite", Port: 80}
		}
		if strings.Contains(pkg, "\"express\"") {
			return Detection{Runtime: "nodejs", Framework: "express", Port: 3000}
		}
		if strings.Contains(pkg, "\"@nestjs/core\"") {
			return Detection{Runtime: "nodejs", Framework: "nestjs", Port: 3000}
		}
		if strings.Contains(pkg, "\"fastify\"") {
			return Detection{Runtime: "nodejs", Framework: "fastify", Port: 3000}
		}
		return Detection{Runtime: "nodejs", Framework: "node", Port: 3000}
	}

	// Python
	if fileExists(repoDir, "requirements.txt") || fileExists(repoDir, "pyproject.toml") || fileExists(repoDir, "Pipfile") {
		if fileExists(repoDir, "manage.py") {
			return Detection{Runtime: "python", Framework: "django", Port: 8000}
		}
		content := readFile(repoDir, "requirements.txt") + readFile(repoDir, "pyproject.toml")
		if strings.Contains(content, "fastapi") {
			return Detection{Runtime: "python", Framework: "fastapi", Port: 8000}
		}
		if strings.Contains(content, "flask") {
			return Detection{Runtime: "python", Framework: "flask", Port: 5000}
		}
		return Detection{Runtime: "python", Framework: "python", Port: 8000}
	}

	// Go
	if fileExists(repoDir, "go.mod") {
		content := readFile(repoDir, "go.mod")
		if strings.Contains(content, "github.com/gin-gonic/gin") {
			return Detection{Runtime: "go", Framework: "gin", Port: 8080}
		}
		if strings.Contains(content, "github.com/gofiber/fiber") {
			return Detection{Runtime: "go", Framework: "fiber", Port: 8080}
		}
		return Detection{Runtime: "go", Framework: "go", Port: 8080}
	}

	// Ruby
	if fileExists(repoDir, "Gemfile") {
		content := readFile(repoDir, "Gemfile")
		if strings.Contains(content, "rails") {
			return Detection{Runtime: "ruby", Framework: "rails", Port: 3000}
		}
		return Detection{Runtime: "ruby", Framework: "ruby", Port: 3000}
	}

	// Java
	if fileExists(repoDir, "pom.xml") {
		return Detection{Runtime: "java", Framework: "maven", Port: 8080}
	}
	if fileExists(repoDir, "build.gradle") || fileExists(repoDir, "build.gradle.kts") {
		return Detection{Runtime: "java", Framework: "gradle", Port: 8080}
	}

	// Rust
	if fileExists(repoDir, "Cargo.toml") {
		return Detection{Runtime: "rust", Framework: "rust", Port: 8080}
	}

	// Static
	if fileExists(repoDir, "index.html") {
		return Detection{Runtime: "static", Framework: "static", Port: 80}
	}

	return Detection{Runtime: "unknown", Framework: "unknown", Port: 3000}
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func readFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return string(data)
}
```

### Step 2: Create `internal/detector/env_vars.go`

**File**: `luxview-engine/internal/detector/env_vars.go`
**Action**: Create

```go
package detector

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`os\.environ\["([A-Z_][A-Z0-9_]*)"\]`),
	regexp.MustCompile(`os\.environ\.get\("([A-Z_][A-Z0-9_]*)"`),
	regexp.MustCompile(`os\.Getenv\("([A-Z_][A-Z0-9_]*)"\)`),
	regexp.MustCompile(`env\("([A-Z_][A-Z0-9_]*)"\)`),
}

// Env vars that are typically set by the platform or runtime, not by the user.
var platformEnvVars = map[string]bool{
	"NODE_ENV": true, "PORT": true, "HOME": true, "PATH": true,
	"CI": true, "PWD": true, "USER": true, "SHELL": true,
	"HOSTNAME": true, "TERM": true, "LANG": true,
}

func detectEnvVars(repoDir string) []agent.EnvHint {
	seen := make(map[string]bool)
	var hints []agent.EnvHint

	// 1. Scan .env.example, .env.sample, .env.template
	for _, name := range []string{".env.example", ".env.sample", ".env.template"} {
		path := filepath.Join(repoDir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key == "" || seen[key] || platformEnvVars[key] {
				continue
			}
			seen[key] = true
			hints = append(hints, agent.EnvHint{
				Key:         key,
				Description: "From " + name,
				Required:    true,
			})
		}
		f.Close()
	}

	// 2. Scan source code for env var references
	scanDir(repoDir, repoDir, seen, &hints)

	return hints
}

func scanDir(baseDir, dir string, seen map[string]bool, hints *[]agent.EnvHint) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, "dist": true, "build": true,
		".next": true, "__pycache__": true, "vendor": true, ".venv": true,
		"target": true, "coverage": true,
	}

	for _, e := range entries {
		if e.IsDir() {
			if skipDirs[e.Name()] {
				continue
			}
			scanDir(baseDir, filepath.Join(dir, e.Name()), seen, hints)
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" &&
			ext != ".py" && ext != ".go" && ext != ".rs" && ext != ".java" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(data) > 64*1024 {
			continue
		}
		content := string(data)

		for _, re := range envPatterns {
			matches := re.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				key := m[1]
				if seen[key] || platformEnvVars[key] {
					continue
				}
				seen[key] = true
				*hints = append(*hints, agent.EnvHint{
					Key:         key,
					Description: "Referenced in source code",
					Required:    false,
				})
			}
		}
	}
}
```

### Step 3: Create `internal/detector/services.go`

**File**: `luxview-engine/internal/detector/services.go`
**Action**: Create

```go
package detector

import (
	"path/filepath"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

func detectServices(repoDir string) []agent.ServiceRecommendation {
	var recs []agent.ServiceRecommendation
	seen := make(map[string]bool)

	pkg := readFile(repoDir, "package.json")
	goMod := readFile(repoDir, "go.mod")
	reqs := readFile(repoDir, "requirements.txt")
	prisma := readFile(repoDir, filepath.Join("prisma", "schema.prisma"))
	compose := readFile(repoDir, "docker-compose.yml") + readFile(repoDir, "docker-compose.yaml")

	// PostgreSQL
	if (strings.Contains(prisma, "postgresql") ||
		strings.Contains(pkg, "\"pg\"") ||
		strings.Contains(pkg, "\"postgres\"") ||
		strings.Contains(pkg, "\"@prisma/client\"") && strings.Contains(prisma, "postgresql") ||
		strings.Contains(goMod, "github.com/lib/pq") ||
		strings.Contains(goMod, "github.com/jackc/pgx") ||
		strings.Contains(reqs, "psycopg2") ||
		strings.Contains(reqs, "asyncpg") ||
		strings.Contains(compose, "postgres")) && !seen["postgres"] {
		seen["postgres"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "PostgreSQL",
			CurrentEvidence:    detectPostgresEvidence(pkg, prisma, goMod, reqs, compose),
			RecommendedService: "postgres",
			Reason:             "Managed PostgreSQL with automatic backups",
			ManualSteps: []string{
				"Set DATABASE_URL in environment variables",
				"Run database migrations",
				"Verify application connects correctly",
			},
		})
	}

	// Redis
	if (strings.Contains(pkg, "\"redis\"") ||
		strings.Contains(pkg, "\"ioredis\"") ||
		strings.Contains(goMod, "github.com/redis/go-redis") ||
		strings.Contains(goMod, "github.com/go-redis/redis") ||
		strings.Contains(reqs, "redis") ||
		strings.Contains(compose, "redis")) && !seen["redis"] {
		seen["redis"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "Redis",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "redis",
			Reason:             "Managed Redis for caching and sessions",
			ManualSteps: []string{
				"Set REDIS_URL in environment variables",
				"Verify cache/session functionality",
			},
		})
	}

	// MongoDB
	if (strings.Contains(pkg, "\"mongoose\"") ||
		strings.Contains(pkg, "\"mongodb\"") ||
		strings.Contains(goMod, "go.mongodb.org/mongo-driver") ||
		strings.Contains(reqs, "pymongo") ||
		strings.Contains(compose, "mongo")) && !seen["mongodb"] {
		seen["mongodb"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "MongoDB",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "mongodb",
			Reason:             "Managed MongoDB instance",
			ManualSteps: []string{
				"Set MONGODB_URL in environment variables",
				"Verify database connectivity",
			},
		})
	}

	// RabbitMQ
	if (strings.Contains(pkg, "\"amqplib\"") ||
		strings.Contains(goMod, "github.com/rabbitmq/amqp091-go") ||
		strings.Contains(reqs, "pika") ||
		strings.Contains(compose, "rabbitmq")) && !seen["rabbitmq"] {
		seen["rabbitmq"] = true
		recs = append(recs, agent.ServiceRecommendation{
			CurrentService:     "RabbitMQ",
			CurrentEvidence:    "Detected in project dependencies",
			RecommendedService: "rabbitmq",
			Reason:             "Managed message queue",
			ManualSteps: []string{
				"Set RABBITMQ_URL in environment variables",
				"Verify queue connectivity",
			},
		})
	}

	return recs
}

func detectPostgresEvidence(pkg, prisma, goMod, reqs, compose string) string {
	if strings.Contains(prisma, "postgresql") {
		return "prisma/schema.prisma: provider = postgresql"
	}
	if strings.Contains(pkg, "\"pg\"") {
		return "package.json: pg dependency"
	}
	if strings.Contains(pkg, "\"postgres\"") {
		return "package.json: postgres dependency"
	}
	if strings.Contains(goMod, "pgx") || strings.Contains(goMod, "lib/pq") {
		return "go.mod: PostgreSQL driver"
	}
	if strings.Contains(reqs, "psycopg2") || strings.Contains(reqs, "asyncpg") {
		return "requirements.txt: PostgreSQL driver"
	}
	return "docker-compose: postgres service"
}
```

### Step 4: Create `internal/detector/templates.go`

**File**: `luxview-engine/internal/detector/templates.go`
**Action**: Create

```go
package detector

import (
	"strings"
)

func generateDockerfile(det Detection, repoDir string) string {
	switch det.Runtime {
	case "nodejs":
		return nodeDockerfile(det, repoDir)
	case "python":
		return pythonDockerfile(det)
	case "go":
		return goDockerfile()
	case "ruby":
		return rubyDockerfile()
	case "java":
		return javaDockerfile(det)
	case "rust":
		return rustDockerfile()
	case "static":
		return staticDockerfile()
	default:
		return nodeDockerfile(det, repoDir) // fallback
	}
}

func nodeDockerfile(det Detection, repoDir string) string {
	// Detect package manager
	pm := "npm"
	lockfile := "package-lock.json"
	installCmd := "npm ci --omit=dev"
	if fileExists(repoDir, "pnpm-lock.yaml") {
		pm = "pnpm"
		lockfile = "pnpm-lock.yaml"
		installCmd = "corepack enable && pnpm install --frozen-lockfile --prod"
	} else if fileExists(repoDir, "yarn.lock") {
		pm = "yarn"
		lockfile = "yarn.lock"
		installCmd = "corepack enable && yarn install --frozen-lockfile --production"
	}

	pkg := readFile(repoDir, "package.json")
	hasBuildScript := strings.Contains(pkg, "\"build\"")

	if det.Framework == "vite" {
		// SPA: build + serve with nginx
		return `# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json ` + lockfile + ` ./
RUN ` + strings.Replace(installCmd, "--prod", "", 1) + `
COPY . .
RUN ` + pm + ` run build

# Production stage
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`
	}

	if det.Framework == "nextjs" {
		return `FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json ` + lockfile + ` ./
RUN ` + strings.Replace(installCmd, "--prod", "", 1) + `
COPY . .
RUN ` + pm + ` run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["` + pm + `", "start"]
`
	}

	// Generic Node.js (Express, NestJS, Fastify, etc.)
	buildStep := ""
	if hasBuildScript {
		buildStep = "RUN " + pm + " run build\n"
	}

	return `FROM node:20-alpine
WORKDIR /app
COPY package.json ` + lockfile + ` ./
RUN ` + installCmd + `
COPY . .
` + buildStep + `EXPOSE ` + itoa(det.Port) + `
CMD ["node", "dist/index.js"]
`
}

func pythonDockerfile(det Detection) string {
	cmd := `CMD ["python", "app.py"]`
	if det.Framework == "django" {
		cmd = `CMD ["gunicorn", "--bind", "0.0.0.0:8000", "config.wsgi:application"]`
	} else if det.Framework == "fastapi" {
		cmd = `CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]`
	} else if det.Framework == "flask" {
		cmd = `CMD ["gunicorn", "--bind", "0.0.0.0:5000", "app:app"]`
	}

	return `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE ` + itoa(det.Port) + `
` + cmd + `
`
}

func goDockerfile() string {
	return `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
`
}

func rubyDockerfile() string {
	return `FROM ruby:3.3-slim
WORKDIR /app
COPY Gemfile Gemfile.lock ./
RUN bundle install --without development test
COPY . .
EXPOSE 3000
CMD ["bundle", "exec", "rails", "server", "-b", "0.0.0.0"]
`
}

func javaDockerfile(det Detection) string {
	if det.Framework == "gradle" {
		return `FROM gradle:8-jdk21-alpine AS builder
WORKDIR /app
COPY . .
RUN gradle build --no-daemon -x test

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=builder /app/build/libs/*.jar app.jar
EXPOSE 8080
CMD ["java", "-jar", "app.jar"]
`
	}
	return `FROM maven:3.9-eclipse-temurin-21-alpine AS builder
WORKDIR /app
COPY . .
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=builder /app/target/*.jar app.jar
EXPOSE 8080
CMD ["java", "-jar", "app.jar"]
`
}

func rustDockerfile() string {
	return `FROM rust:1.77-alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY . .
RUN cargo build --release

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/target/release/* .
EXPOSE 8080
CMD ["./app"]
`
}

func staticDockerfile() string {
	return `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`
}

func itoa(n int) string {
	return strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						strings.Replace(
							"00000",
							"00000", "", 1,
						), "", "", 0,
					), "", "", 0,
				), "", "", 0,
			), "", "", 0,
		), "0",
	), "")
	// Actually, just use fmt.Sprintf
}
```

Wait — Go doesn't have a simple `itoa` in strings. Use `strconv.Itoa`:

```go
import "strconv"

// Replace itoa calls with strconv.Itoa
```

---

## Phase C: Backend — Update Analyze to use detector fallback

Already covered in Phase A Step 7. The `Analyze` handler will:
1. Try `getAIConfig()`
2. If err (AI disabled) → `detector.Analyze(cloneDir)`
3. If ok → existing AI flow

Add import `"github.com/luxview/engine/internal/detector"` to `analyze_handler.go`.

---

## Phase D: Frontend — Simplify ServiceRecommendationCard

### Step 1: Remove "auto" mode, keep only "provision" and "ignore"

**File**: `luxview-dashboard/src/components/deploy/ServiceRecommendationCard.tsx`
**Action**: Modify

Change `MigrationMode` type:
```typescript
export type MigrationMode = 'provision' | 'ignore';
```

Remove the 3-button selector. Replace with a simpler toggle:
- **Provision** (default): "Provision this service and inject env vars"
- **Skip**: "I'll configure this myself"

Remove the "auto" button entirely and the manual steps collapsible (manual steps only applied to the old code-generation flow).

---

## Phase E: Frontend — Simplify DeployAnalysis + DeployWizard + NewApp

### Step 1: Simplify `analyze.ts`

**File**: `luxview-dashboard/src/api/analyze.ts`
**Action**: Modify

Remove:
- `autoMigrate` method
- `AutoMigrateResult` interface
- `CodeChange` interface
- `codeChanges` field from `ServiceRecommendation`

Add:
```typescript
export interface ApplyAnalysisRequest {
  dockerfile: string;
  envVars: Record<string, string>;
  services: string[]; // ["postgres", "redis"]
}

export interface ApplyAnalysisResponse {
  message: string;
  provisionedEnvs?: Record<string, string>;
}

// In analyzeApi:
async applyAnalysis(appId: string, req: ApplyAnalysisRequest): Promise<ApplyAnalysisResponse> {
  const { data } = await api.post<ApplyAnalysisResponse>(`/apps/${appId}/apply-analysis`, req);
  return data;
},
```

### Step 2: Simplify `DeployAnalysis.tsx`

**File**: `luxview-dashboard/src/components/deploy/DeployAnalysis.tsx`
**Action**: Modify

Remove:
- `DeployFlowAnimation` component entirely (lines 131-219) — no more auto-migrate progress
- `deploying` state handling that shows the animation
- The `deploying` prop

Change `onApprove` signature:
```typescript
onApprove: (dockerfile: string, envVars: Record<string, string>, services: string[]) => void;
```

The approve button now collects selected service types (where mode === 'provision') as a string array instead of the full serviceModes record.

### Step 3: Simplify `NewApp.tsx`

**File**: `luxview-dashboard/src/pages/NewApp.tsx`
**Action**: Modify

Remove:
- `prUrls` state and `setPrUrls`
- `provisioningDone` state
- `analyzeApi.autoMigrate()` call loop in `handleApproveAndProvision`

Replace `handleApproveAndProvision` with `handleApproveAnalysis`:
```typescript
const handleApproveAnalysis = async (dockerfile: string, envVars: Record<string, string>, services: string[]) => {
  const appId = createdAppIdRef.current;
  if (!appId) return;

  setDeploying(true);
  try {
    await analyzeApi.applyAnalysis(appId, { dockerfile, envVars, services });
    addNotification({
      type: 'success',
      title: t('analyze.analysisApplied'),
    });
    // Go to review step
    // setStep handled by wizard
  } catch {
    addNotification({
      type: 'error',
      title: t('app.notifications.deploymentFailed'),
    });
  } finally {
    setDeploying(false);
  }
};
```

Remove `prUrls` and `provisioningDone` props from `DeployWizard`.

### Step 4: Simplify `DeployWizard.tsx`

**File**: `luxview-dashboard/src/components/deploy/DeployWizard.tsx`
**Action**: Modify

Remove:
- `prUrls` and `provisioningDone` props
- PR summary step (step 4 when PRs exist)
- All PR-related UI (GitPullRequest, ExternalLink icons)
- `hasPRs` logic
- `prSummaryStep` variable

Steps become fixed:
1. Select Repository
2. Configure
3. Environment
4. AI Analysis
5. Review & Deploy

The auto-advance from step 3 to step 4 after provisioning is no longer needed. Instead, `onDeploy` in step 3 calls `applyAnalysis` and on success advances to review step.

---

## Phase F: Frontend — Respect AI toggle in wizard

### Step 1: Check AI settings in NewApp

**File**: `luxview-dashboard/src/pages/NewApp.tsx`
**Action**: Modify

Add state:
```typescript
const [aiEnabled, setAiEnabled] = useState<boolean | null>(null);

useEffect(() => {
  aiSettingsApi.get()
    .then((s) => setAiEnabled(s.aiEnabled))
    .catch(() => setAiEnabled(false));
}, []);
```

Pass `aiEnabled` to `DeployWizard`.

### Step 2: Skip AI step when disabled

**File**: `luxview-dashboard/src/components/deploy/DeployWizard.tsx`
**Action**: Modify

Add `aiEnabled` prop. When `aiEnabled === false`:
- Remove "AI Analysis" from steps array
- In `handleNext`, when on step 2 (environment), skip to review instead of triggering analysis
- The wizard goes: Repository → Configure → Environment → Review & Deploy

```typescript
const steps = aiEnabled
  ? [selectRepo, configure, environment, aiAnalysis, reviewDeploy]
  : [selectRepo, configure, environment, reviewDeploy];
```

When `aiEnabled === false` and user is on Environment step, clicking "Continue" goes directly to Review.

---

## Testing

1. **AI enabled**: New app → Repository → Configure → Env → AI Analysis (shows Dockerfile + env vars + services) → Approve → provisions services + saves Dockerfile → Review → Deploy
2. **AI disabled**: New app → Repository → Configure → Env → Review → Deploy (no analysis step)
3. **Analyze-failure**: Unchanged behavior on AppDetail page
4. **Deterministic detection**: Disable AI → create app with Node.js/Next.js repo → verify Dockerfile template is correct
5. **Service provisioning**: Select services in analysis → approve → verify services created and env vars injected

---

## Key Details

- **Import to add in `analyze_handler.go`**: `"github.com/luxview/engine/internal/detector"`, `"github.com/luxview/engine/internal/model"`, `"github.com/luxview/engine/internal/service"`
- **Import to add in `router.go`**: Remove auto_migrate handler initialization
- **`AppRepo.UpdateEnvVars`**: Verify this method exists; if not, use `appsApi.updateEnvVars` on frontend side instead
- **`ServiceRepo.FindByAppAndType`**: Already used in auto_migrate.go, keep the method
- **Provisioner env injection**: The `Provisioner.Provision()` already injects `DATABASE_URL` etc. into the app's env vars automatically — no extra logic needed
- **Translation keys to add**: `analyze.analysisApplied`, `analyze.provision`, `analyze.skipService`
- **Translation keys to remove**: `analyze.autoDescription`, `analyze.migrationMode.auto`, `deploy.wizard.steps.prSummary`, `deploy.wizard.prSummary.*`
