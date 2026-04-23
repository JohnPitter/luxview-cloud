package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/luxview/engine/internal/buildpack"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// DeployRequest holds the data needed to deploy an app.
type DeployRequest struct {
	AppID     uuid.UUID
	UserID    uuid.UUID
	CommitSHA string
	CommitMsg string
	Source    string // "manual", "webhook", "rollback" — empty defaults to auto/ai detection
}

// Deployer orchestrates the full deploy flow.
type Deployer struct {
	appRepo        *repository.AppRepo
	deployRepo     *repository.DeploymentRepo
	userRepo       *repository.UserRepo
	serviceRepo    *repository.ServiceRepo
	settingsRepo   *repository.SettingsRepo
	provisioner    *Provisioner
	detector       *Detector
	builder        *Builder
	container      *ContainerManager
	portManager    *PortManager
	healthChecker  *HealthChecker
	docker         *dockerclient.Client
	encryptionKey  []byte
	buildTimeout   time.Duration
	appLocks       sync.Map // per-app deploy lock to prevent concurrent deploys
}

func NewDeployer(
	appRepo *repository.AppRepo,
	deployRepo *repository.DeploymentRepo,
	userRepo *repository.UserRepo,
	serviceRepo *repository.ServiceRepo,
	settingsRepo *repository.SettingsRepo,
	provisioner *Provisioner,
	docker *dockerclient.Client,
	portManager *PortManager,
	encryptionKey []byte,
	buildTimeout time.Duration,
	appNetwork string,
) *Deployer {
	container := NewContainerManager(docker, appNetwork)
	return &Deployer{
		appRepo:       appRepo,
		deployRepo:    deployRepo,
		userRepo:      userRepo,
		serviceRepo:   serviceRepo,
		settingsRepo:  settingsRepo,
		provisioner:   provisioner,
		detector:      NewDetector(),
		builder:       NewBuilder(docker),
		container:     container,
		portManager:   portManager,
		healthChecker: NewHealthChecker(appRepo, container),
		docker:        docker,
		encryptionKey: encryptionKey,
		buildTimeout:  buildTimeout,
	}
}

// Deploy executes the full deploy pipeline for an app.
// Uses a per-app lock to prevent concurrent deploys of the same app.
func (d *Deployer) Deploy(ctx context.Context, req DeployRequest) error {
	// Per-app lock: if another deploy is already running for this app, skip
	if _, loaded := d.appLocks.LoadOrStore(req.AppID.String(), true); loaded {
		log := logger.With("deployer")
		log.Warn().Str("app_id", req.AppID.String()).Msg("deploy already in progress, skipping duplicate")
		return fmt.Errorf("deploy already in progress for app %s", req.AppID.String())
	}
	defer d.appLocks.Delete(req.AppID.String())

	log := logger.With("deployer")
	start := time.Now()

	log.Debug().
		Str("app_id", req.AppID.String()).
		Str("user_id", req.UserID.String()).
		Str("commit_sha", req.CommitSHA).
		Msg("deploy started")

	app, err := d.appRepo.FindByID(ctx, req.AppID)
	if err != nil || app == nil {
		return fmt.Errorf("app not found: %w", err)
	}

	// Determine deploy source
	hasCustomDockerfile := app.CustomDockerfile != nil && *app.CustomDockerfile != ""
	deploySource := req.Source
	if deploySource == "" {
		deploySource = "auto"
	}

	log.Debug().
		Str("app_subdomain", app.Subdomain).
		Str("repo_url", app.RepoURL).
		Str("repo_branch", app.RepoBranch).
		Bool("has_custom_dockerfile", hasCustomDockerfile).
		Str("deploy_source", deploySource).
		Msg("app loaded")

	// Create deployment record
	deployment := &model.Deployment{
		AppID:         app.ID,
		CommitSHA:     req.CommitSHA,
		CommitMessage: req.CommitMsg,
		Status:        model.DeployBuilding,
		ImageTag:      fmt.Sprintf("luxview/%s:%s", app.Subdomain, req.CommitSHA[:min(7, len(req.CommitSHA))]),
		Source:        deploySource,
	}
	if err := d.deployRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}

	// Update app status
	if err := d.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusBuilding, app.ContainerID); err != nil {
		log.Error().Err(err).Msg("failed to update app status")
	}

	// Clone repo
	buildDir := filepath.Join(os.TempDir(), "luxview-builds", deployment.ID.String())
	defer os.RemoveAll(buildDir)

	if err := d.cloneRepo(ctx, app, buildDir); err != nil {
		d.failDeploy(ctx, deployment, app, "clone failed: "+err.Error(), start)
		return err
	}

	log.Debug().Str("build_dir", buildDir).Msg("repo cloned, build directory ready")

	// Detect stack: custom Dockerfile vs auto-detect
	var bp buildpack.Buildpack

	if app.CustomDockerfile != nil && *app.CustomDockerfile != "" {
		// Write custom Dockerfile from AI agent / user into the build dir
		dockerfilePath := filepath.Join(buildDir, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, []byte(*app.CustomDockerfile), 0644); err != nil {
			d.failDeploy(ctx, deployment, app, "failed to write custom Dockerfile: "+err.Error(), start)
			return fmt.Errorf("write custom dockerfile: %w", err)
		}
		preview := *app.CustomDockerfile
		if len(preview) > 100 {
			preview = preview[:100]
		}
		log.Debug().Str("app", app.Subdomain).Str("dockerfile_preview", preview).Msg("custom Dockerfile written to build dir")
		log.Info().Str("app", app.Subdomain).Msg("using custom Dockerfile — skipping auto-detect")

		// Use DockerfilePack directly (skip detector which would reject monorepos)
		dfp := &buildpack.DockerfilePack{}
		dfp.DetectPort(buildDir)
		bp = dfp
	} else {
		// Auto-detect stack
		result := d.detector.Detect(buildDir)
		if result == nil {
			failMsg := "no supported stack detected — run AI analysis to generate a custom Dockerfile"
			if d.detector.IsMonorepo(buildDir) {
				failMsg = "monorepo detected (turbo/pnpm/lerna) — workspace:* dependencies require a custom Dockerfile. Run AI analysis first to generate one."
			}
			d.failDeploy(ctx, deployment, app, failMsg, start)
			return fmt.Errorf("no buildpack detected for app %s", app.Subdomain)
		}
		bp = result.Buildpack
		buildDir = result.BuildDir
	}

	log.Debug().Str("buildpack", bp.Name()).Str("build_dir", buildDir).Msg("stack detected")

	// If using a custom Dockerfile, detect the EXPOSE port
	if dfp, ok := bp.(*buildpack.DockerfilePack); ok {
		dfp.DetectPort(buildDir)
		log.Debug().Str("app", app.Subdomain).Int("detected_port", bp.DefaultPort()).Msg("DockerfilePack port detected")
	}

	// Update app stack
	app.Stack = bp.Name()

	// Build image
	buildCtx, cancel := context.WithTimeout(ctx, d.buildTimeout)
	defer cancel()

	log.Debug().Str("image_tag", deployment.ImageTag).Dur("build_timeout", d.buildTimeout).Msg("starting image build")

	buildLog, err := d.builder.Build(buildCtx, buildDir, bp, deployment.ImageTag)
	if err != nil {
		d.failDeploy(ctx, deployment, app, buildLog+"\n"+err.Error(), start)
		return fmt.Errorf("build failed: %w", err)
	}

	log.Debug().Str("image_tag", deployment.ImageTag).Int("build_log_size", len(buildLog)).Msg("image build succeeded")

	// Update deployment status
	deployment.Status = model.DeployDeploying
	_ = d.deployRepo.UpdateStatus(ctx, deployment.ID, model.DeployDeploying, buildLog, 0)

	// Allocate port if needed
	if app.AssignedPort == 0 {
		port, err := d.portManager.Allocate(ctx)
		if err != nil {
			d.failDeploy(ctx, deployment, app, "port allocation failed: "+err.Error(), start)
			return err
		}
		app.AssignedPort = port
		if err := d.appRepo.UpdatePort(ctx, app.ID, port); err != nil {
			d.failDeploy(ctx, deployment, app, "save port failed: "+err.Error(), start)
			return err
		}
	}

	// Set internal port from buildpack
	app.InternalPort = bp.DefaultPort()

	// Decrypt env vars
	envVars := make(map[string]string)
	if len(app.EnvVars) > 0 {
		// EnvVars is stored as JSON-encoded string (e.g., "\"encrypted_data\"")
		var encrypted string
		if err := json.Unmarshal(app.EnvVars, &encrypted); err == nil {
			decrypted, err := crypto.Decrypt(encrypted, d.encryptionKey)
			if err == nil {
				_ = json.Unmarshal([]byte(decrypted), &envVars)
			} else {
				log.Warn().Err(err).Msg("failed to decrypt env vars")
			}
		}
	}

	// Auto-provision storage if the Dockerfile references STORAGE_PATH but no storage service exists.
	// This ensures apps that need file storage get it automatically on first deploy.
	if hasCustomDockerfile {
		dockerfile := *app.CustomDockerfile
		if strings.Contains(dockerfile, "STORAGE_PATH") || strings.Contains(dockerfile, "/storage") {
			existingStorage, _ := d.serviceRepo.FindByAppAndType(ctx, app.ID, model.ServiceStorage)
			if existingStorage == nil {
				if svc, provErr := d.provisioner.Provision(ctx, app.ID, model.ServiceStorage); provErr == nil {
					log.Info().Str("app", app.Subdomain).Str("service_id", svc.ID.String()).Msg("auto-provisioned storage service")
				} else if !strings.Contains(provErr.Error(), "already provisioned") {
					log.Warn().Err(provErr).Str("app", app.Subdomain).Msg("failed to auto-provision storage")
				}
			}
		}
	}

	// Inject service env vars (DATABASE_URL, REDIS_URL, etc.) and collect bind mounts.
	// Service vars are injected first, then user env vars override them.
	// This ensures user-defined DATABASE_URL takes priority.
	serviceEnvVars := make(map[string]string)
	var storageBinds []string
	services, err := d.serviceRepo.ListByAppID(ctx, app.ID)
	if err == nil {
		for _, svc := range services {
			var encSvc string
			if err := json.Unmarshal(svc.Credentials, &encSvc); err == nil {
				if decrypted, err := crypto.Decrypt(encSvc, d.encryptionKey); err == nil {
					var creds map[string]string
					if err := json.Unmarshal([]byte(decrypted), &creds); err == nil {
						for k, v := range d.provisioner.GetEnvVarsForService(&svc, creds) {
							serviceEnvVars[k] = v
						}
						if binds := d.provisioner.GetStorageBinds(&svc, creds); len(binds) > 0 {
							storageBinds = append(storageBinds, binds...)
						}
					}
				}
			}
		}
		if len(services) > 0 {
			svcKeys := make([]string, 0, len(serviceEnvVars))
			for k := range serviceEnvVars {
				svcKeys = append(svcKeys, k)
			}
			log.Info().Int("count", len(services)).Strs("env_keys", svcKeys).Int("binds", len(storageBinds)).Msg("injected service env vars")
		}
	}

	// Merge: service env vars first, then user env vars override.
	// Empty user values are dropped entirely — strict config validators (Zod enum,
	// class-validator, pydantic) reject "" and most frameworks treat missing as
	// "use default", so omitting the key is safer than forwarding an empty string.
	mergedEnvVars := make(map[string]string)
	for k, v := range serviceEnvVars {
		mergedEnvVars[k] = v
	}
	for k, v := range envVars {
		if v == "" {
			continue
		}
		mergedEnvVars[k] = v
	}
	envVars = mergedEnvVars

	// Inject platform timezone if configured and not already set by user
	if _, hasTZ := envVars["TZ"]; !hasTZ {
		if tz, err := d.settingsRepo.Get(ctx, "platform_timezone"); err == nil && tz != "" {
			envVars["TZ"] = tz
		}
	}

	// Stop old container (blue-green)
	oldContainerID := app.ContainerID
	if oldContainerID != "" {
		_ = d.container.Stop(ctx, oldContainerID)
	}

	log.Debug().
		Int("assigned_port", app.AssignedPort).
		Int("internal_port", app.InternalPort).
		Int("env_vars", len(envVars)).
		Int("service_env_vars", len(serviceEnvVars)).
		Msg("starting container")

	// Start new container
	containerID, err := d.container.Start(ctx, app, deployment.ImageTag, envVars, storageBinds)
	if err != nil {
		// Try to restart old container on failure
		if oldContainerID != "" {
			_ = d.container.Restart(ctx, oldContainerID)
		}
		d.failDeploy(ctx, deployment, app, buildLog+"\n\n--- DEPLOY FAILED ---\ncontainer start failed: "+err.Error(), start)
		return fmt.Errorf("start container: %w", err)
	}

	log.Debug().Str("container_id", containerID[:min(12, len(containerID))]).Msg("container started")

	// Run post-deploy hooks BEFORE health check so migrations create tables
	// before the app's workers try to query them.
	d.runPostDeployHooks(ctx, containerID, bp.Name())

	// Health check — longer timeout for slow-starting stacks (Java, etc.)
	healthTimeout := 120 * time.Second
	switch bp.Name() {
	case "java", "nextjs", "dockerfile":
		healthTimeout = 180 * time.Second
	}

	log.Debug().Dur("health_timeout", healthTimeout).Str("stack", bp.Name()).Msg("starting health check")

	healthStart := time.Now()
	healthy := d.healthChecker.WaitForHealthy(ctx, app.ID, containerID, app.InternalPort, app.AssignedPort, healthTimeout)
	if !healthy {
		// Capture container logs to help user diagnose the issue
		failReason := "health check failed"
		containerLogs, logErr := d.docker.ContainerLogs(ctx, containerID, "30")
		if logErr == nil {
			logBytes, _ := io.ReadAll(containerLogs)
			containerLogs.Close()
			if len(logBytes) > 0 {
				cleaned := stripDockerHeaders(logBytes)
				if len(cleaned) > 0 {
					failReason = fmt.Sprintf("health check failed — container logs:\n%s", cleaned)
				}
			}
		}

		log.Debug().Int("fail_reason_len", len(failReason)).Msg("health check failed, rolling back")

		// Append failure reason to build log so the user sees both
		fullLog := buildLog + "\n\n--- DEPLOY FAILED ---\n" + failReason

		// Rollback: stop new, restart old
		_ = d.container.Stop(ctx, containerID)
		_ = d.container.Remove(ctx, containerID)
		if oldContainerID != "" {
			_ = d.container.Restart(ctx, oldContainerID)
			_ = d.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, oldContainerID)
		}
		d.failDeploy(ctx, deployment, app, fullLog, start)
		return fmt.Errorf("health check failed for app %s", app.Subdomain)
	}

	log.Debug().Dur("health_check_duration", time.Since(healthStart)).Msg("health check passed")

	// Remove old container
	if oldContainerID != "" {
		_ = d.container.Remove(ctx, oldContainerID)
	}

	// Finalize
	duration := int(time.Since(start).Milliseconds())
	app.ContainerID = containerID
	app.Status = model.AppStatusRunning
	if err := d.appRepo.Update(ctx, app); err != nil {
		log.Error().Err(err).Msg("failed to update app after deploy")
	}

	if err := d.deployRepo.UpdateStatus(ctx, deployment.ID, model.DeployLive, buildLog, duration); err != nil {
		log.Error().Err(err).Msg("failed to update deployment status")
	}

	log.Info().
		Str("app", app.Subdomain).
		Str("deployment", deployment.ID.String()).
		Int("duration_ms", duration).
		Msg("deploy completed")

	return nil
}

// runPostDeployHooks executes post-deploy commands inside the container.
// Phase 1: Detect ORM/migration tools and run schema migrations.
// Phase 2: Detect and run seed scripts to populate initial data.
func (d *Deployer) runPostDeployHooks(ctx context.Context, containerID string, stack string) {
	log := logger.With("deployer")

	log.Debug().Str("stack", stack).Msg("checking migration tools for post-deploy hooks")

	// === Phase 1: Migrations ===
	d.runMigrations(ctx, containerID, log)

	// === Phase 2: Seeds ===
	d.runSeeds(ctx, containerID, log)
}

// runMigrations detects ORM/migration tools and pushes schema to DB.
func (d *Deployer) runMigrations(ctx context.Context, containerID string, log zerolog.Logger) {
	// First, check if package.json has a db:migrate script (monorepos, custom setups)
	if d.tryHook(ctx, containerID, log, "package.json db:migrate",
		[]string{"sh", "-c", "cat package.json 2>/dev/null | grep -q '\"db:migrate\"'"},
		[]string{"npm", "run", "db:migrate"}) {
		return
	}

	// Also check for "migrate" script
	if d.tryHook(ctx, containerID, log, "package.json migrate",
		[]string{"sh", "-c", "cat package.json 2>/dev/null | grep -q '\"migrate\"'"},
		[]string{"npm", "run", "migrate"}) {
		return
	}

	// Prisma — apply schema to DB via migrate deploy (preferred) or db push (fallback).
	// Supports root and monorepo layouts (packages/*/prisma/schema.prisma).
	// In prod containers, "prisma" CLI (devDep) is removed but @prisma/client remains.
	// We detect @prisma/client version and use npx prisma@<version> to run commands.
	// IMPORTANT for pnpm monorepos: require('@prisma/client/package.json') only resolves
	// from within the package that declares it as a dependency, NOT from the monorepo root.
	if d.tryHook(ctx, containerID, log, "prisma migrate",
		[]string{"sh", "-c", "test -f prisma/schema.prisma || ls packages/*/prisma/schema.prisma >/dev/null 2>&1"},
		[]string{"sh", "-c", `
			SCHEMA=$(find /app -path "*/prisma/schema.prisma" -not -path "*/node_modules/*" | head -1)
			if [ -z "$SCHEMA" ]; then exit 0; fi
			SCHEMA_DIR=$(dirname "$SCHEMA")
			PACKAGE_DIR=$(dirname "$SCHEMA_DIR")

			# Resolve prisma CLI: try direct CLI, .pnpm store, then npx with pinned version
			PRISMA_CMD=""

			# 1) Try existing prisma CLI (available if prisma is a prod dep or not pruned)
			if command -v prisma >/dev/null 2>&1; then
				PRISMA_CMD="prisma"
			fi

			# 2) Try finding prisma CLI in .pnpm store
			if [ -z "$PRISMA_CMD" ]; then
				PRISMA_CLI=$(find /app/node_modules/.pnpm -name "prisma" -path "*/node_modules/.bin/prisma" 2>/dev/null | head -1)
				if [ -n "$PRISMA_CLI" ]; then
					PRISMA_CMD="$PRISMA_CLI"
				fi
			fi

			# 3) Detect @prisma/client version — try from package dir first (pnpm monorepo),
			#    then from root (standard layout)
			if [ -z "$PRISMA_CMD" ]; then
				PRISMA_VER=""
				if [ "$PACKAGE_DIR" != "$SCHEMA_DIR" ] && [ -d "$PACKAGE_DIR/node_modules" ]; then
					PRISMA_VER=$(cd "$PACKAGE_DIR" && node -e "try{console.log(require('@prisma/client/package.json').version)}catch(e){}" 2>/dev/null)
				fi
				if [ -z "$PRISMA_VER" ]; then
					PRISMA_VER=$(node -e "try{console.log(require('@prisma/client/package.json').version)}catch(e){}" 2>/dev/null)
				fi
				if [ -n "$PRISMA_VER" ]; then
					echo "Using npx prisma@$PRISMA_VER (pinned to @prisma/client version)"
					PRISMA_CMD="npx --yes prisma@$PRISMA_VER"
				fi
			fi

			if [ -z "$PRISMA_CMD" ]; then
				echo "prisma CLI not found and version not detectable, skipping"
				exit 0
			fi

			# If migrations folder exists, use migrate deploy (production-safe, applies pending migrations).
			# Otherwise fall back to db push (schema-only sync, no migration history).
			if [ -d "$SCHEMA_DIR/migrations" ]; then
				echo "Running: $PRISMA_CMD migrate deploy --schema=$SCHEMA"
				OUTPUT=$($PRISMA_CMD migrate deploy --schema="$SCHEMA" 2>&1)
				EXIT_CODE=$?
				echo "$OUTPUT"

				# P3005 = non-empty DB without _prisma_migrations table (first deploy on existing DB).
				# Strategy: sync schema with db push, then baseline all migrations so future deploys work.
				if [ $EXIT_CODE -ne 0 ] && echo "$OUTPUT" | grep -q "P3005"; then
					echo "Detected P3005: syncing schema with db push, then baselining..."
					$PRISMA_CMD db push --schema="$SCHEMA" --skip-generate --accept-data-loss 2>&1
					for m in $(ls "$SCHEMA_DIR/migrations/" | grep -v migration_lock.toml | sort); do
						echo "  Resolving: $m"
						$PRISMA_CMD migrate resolve --applied "$m" --schema="$SCHEMA" 2>&1
					done
					echo "Schema synced and migrations baselined."
				else
					exit $EXIT_CODE
				fi
			else
				echo "Running: $PRISMA_CMD db push --schema=$SCHEMA"
				$PRISMA_CMD db push --schema="$SCHEMA" --skip-generate --accept-data-loss
			fi
		`}) {
		return
	}

	// Drizzle — push schema to DB
	if d.tryHook(ctx, containerID, log, "drizzle-kit push",
		[]string{"sh", "-c", "test -f node_modules/drizzle-kit/bin.cjs"},
		[]string{"npx", "drizzle-kit", "push"}) {
		return
	}

	// TypeORM — run migrations
	if d.tryHook(ctx, containerID, log, "typeorm migrations",
		[]string{"sh", "-c", "test -f node_modules/typeorm/cli.js"},
		[]string{"npx", "typeorm", "migration:run", "-d", "dist/data-source.js"}) {
		return
	}

	// Knex — run migrations
	if d.tryHook(ctx, containerID, log, "knex migrate",
		[]string{"sh", "-c", "test -f node_modules/.bin/knex"},
		[]string{"npx", "knex", "migrate:latest"}) {
		return
	}

	// Python — Django migrate
	if d.tryHook(ctx, containerID, log, "django migrate",
		[]string{"sh", "-c", "test -f manage.py"},
		[]string{"python", "manage.py", "migrate", "--noinput"}) {
		return
	}

	// Python — Alembic
	if d.tryHook(ctx, containerID, log, "alembic upgrade",
		[]string{"sh", "-c", "test -f alembic.ini"},
		[]string{"alembic", "upgrade", "head"}) {
		return
	}

	// Go — golang-migrate (binary installed or in PATH)
	if d.tryHook(ctx, containerID, log, "golang-migrate",
		[]string{"sh", "-c", "command -v migrate >/dev/null 2>&1 && ls migrations/*.sql >/dev/null 2>&1"},
		[]string{"sh", "-c", `migrate -path ./migrations -database "$DATABASE_URL" up`}) {
		return
	}

	// Go — goose
	if d.tryHook(ctx, containerID, log, "goose migrate",
		[]string{"sh", "-c", "command -v goose >/dev/null 2>&1 && ls migrations/*.sql >/dev/null 2>&1"},
		[]string{"sh", "-c", `goose -dir ./migrations postgres "$DATABASE_URL" up`}) {
		return
	}

	// Go — atlas
	if d.tryHook(ctx, containerID, log, "atlas schema apply",
		[]string{"sh", "-c", "command -v atlas >/dev/null 2>&1"},
		[]string{"sh", "-c", `atlas schema apply --url "$DATABASE_URL" --auto-approve`}) {
		return
	}

	// Java — Flyway (Spring Boot auto-runs on startup, but standalone Flyway CLI also supported)
	if d.tryHook(ctx, containerID, log, "flyway migrate",
		[]string{"sh", "-c", "command -v flyway >/dev/null 2>&1"},
		[]string{"sh", "-c", `flyway -url="$DATABASE_URL" migrate`}) {
		return
	}

	// Java — Liquibase
	if d.tryHook(ctx, containerID, log, "liquibase update",
		[]string{"sh", "-c", "command -v liquibase >/dev/null 2>&1"},
		[]string{"sh", "-c", `liquibase --url="$DATABASE_URL" update`}) {
		return
	}

	// Rust — sqlx
	if d.tryHook(ctx, containerID, log, "sqlx migrate",
		[]string{"sh", "-c", "command -v sqlx >/dev/null 2>&1 && ls migrations/*.sql >/dev/null 2>&1"},
		[]string{"sh", "-c", `sqlx database setup --database-url "$DATABASE_URL"`}) {
		return
	}

	// Rust — diesel
	if d.tryHook(ctx, containerID, log, "diesel migrate",
		[]string{"sh", "-c", "command -v diesel >/dev/null 2>&1 && test -f diesel.toml"},
		[]string{"sh", "-c", `diesel migration run --database-url "$DATABASE_URL"`}) {
		return
	}

	// Generic — SQL migration files with psql (last resort)
	if d.tryHook(ctx, containerID, log, "generic SQL migrations",
		[]string{"sh", "-c", "command -v psql >/dev/null 2>&1 && ls migrations/*.sql >/dev/null 2>&1"},
		[]string{"sh", "-c", `
			echo "Running generic SQL migrations..."
			for f in $(ls migrations/*.sql | sort); do
				echo "Applying $f"
				psql "$DATABASE_URL" -f "$f"
			done
		`}) {
		return
	}

	log.Debug().Msg("no migration tool detected, skipping migrations")
}

// runSeeds detects seed scripts and runs them to populate initial data.
// Seeds run after migrations so tables exist. Seeds should be idempotent (use upsert).
func (d *Deployer) runSeeds(ctx context.Context, containerID string, log zerolog.Logger) {
	// Check package.json for common seed script names
	seedScripts := []string{"db:seed", "seed", "prisma:seed"}
	for _, script := range seedScripts {
		if d.tryHook(ctx, containerID, log, "package.json "+script,
			[]string{"sh", "-c", fmt.Sprintf(`cat package.json 2>/dev/null | grep -q '"%s"'`, script)},
			[]string{"npm", "run", script}) {
			return
		}
	}

	// Prisma db seed (uses the "prisma.seed" field in package.json)
	// Works in monorepos: check root and packages/*/package.json
	if d.tryHook(ctx, containerID, log, "prisma db seed",
		[]string{"sh", "-c", `grep -rq '"seed"' package.json 2>/dev/null && grep -q '"prisma"' package.json 2>/dev/null`},
		[]string{"sh", "-c", `
			# Try npx prisma db seed, with version pinning fallback
			if command -v prisma >/dev/null 2>&1; then
				prisma db seed
				exit $?
			fi
			PRISMA_CLI=$(find /app/node_modules/.pnpm -name "prisma" -path "*/node_modules/.bin/prisma" 2>/dev/null | head -1)
			if [ -n "$PRISMA_CLI" ]; then
				"$PRISMA_CLI" db seed
				exit $?
			fi
			PRISMA_VER=$(node -e "try{console.log(require('@prisma/client/package.json').version)}catch(e){}" 2>/dev/null)
			if [ -n "$PRISMA_VER" ]; then
				npx prisma@$PRISMA_VER db seed
				exit $?
			fi
			echo "prisma CLI not found for seed, skipping"
		`}) {
		return
	}

	// Prisma seed.ts/seed.js — direct execution (monorepo: find in packages/*/prisma/)
	// This handles cases where prisma.seed isn't in package.json but seed file exists
	if d.tryHook(ctx, containerID, log, "prisma seed file",
		[]string{"sh", "-c", "find /app -path '*/prisma/seed.*' -not -path '*/node_modules/*' | grep -q ."},
		[]string{"sh", "-c", `
			SEED=$(find /app -path "*/prisma/seed.*" -not -path "*/node_modules/*" | head -1)
			if [ -z "$SEED" ]; then exit 1; fi
			SEED_DIR=$(dirname $(dirname "$SEED"))
			echo "Running seed: $SEED (from $SEED_DIR)"
			cd "$SEED_DIR"
			if echo "$SEED" | grep -q '\.ts$'; then
				# TypeScript seed — try tsx, ts-node, or compile and run
				if command -v tsx >/dev/null 2>&1; then
					tsx "$SEED"
				elif command -v ts-node >/dev/null 2>&1; then
					ts-node "$SEED"
				else
					# No TS runner in prod — look for compiled JS equivalent
					JS_SEED=$(echo "$SEED" | sed 's|/prisma/seed\.ts|/dist/prisma/seed.js|; s|/src/seed\.ts|/dist/seed.js|')
					if [ -f "$JS_SEED" ]; then
						node "$JS_SEED"
					else
						# Inline execution with @prisma/client
						node --input-type=module -e "$(cat "$SEED" | sed 's/import.*from.*prisma\/client.*/import { PrismaClient } from "@prisma\/client";/')"
					fi
				fi
			else
				node "$SEED"
			fi
		`}) {
		return
	}

	// Django seed/loaddata
	if d.tryHook(ctx, containerID, log, "django loaddata",
		[]string{"sh", "-c", "test -f manage.py && ls */fixtures/*.json >/dev/null 2>&1"},
		[]string{"sh", "-c", "for f in $(find . -path '*/fixtures/*.json'); do python manage.py loaddata $f; done"}) {
		return
	}

	// Go — seed binary or main function
	if d.tryHook(ctx, containerID, log, "go seed",
		[]string{"sh", "-c", "test -f seed 2>/dev/null || test -f cmd/seed/main.go 2>/dev/null"},
		[]string{"sh", "-c", `
			if [ -f seed ]; then ./seed; exit $?; fi
			if [ -f cmd/seed/main.go ]; then go run ./cmd/seed; exit $?; fi
		`}) {
		return
	}

	// Generic — seed.sql file applied via psql
	if d.tryHook(ctx, containerID, log, "seed.sql",
		[]string{"sh", "-c", "command -v psql >/dev/null 2>&1 && test -f seed.sql"},
		[]string{"sh", "-c", `echo "Running seed.sql..." && psql "$DATABASE_URL" -f seed.sql`}) {
		return
	}

	log.Debug().Msg("no seed script detected, skipping seeds")
}

// tryHook checks if a tool is present and runs the associated command.
// Returns true if the tool was detected (regardless of command success/failure).
func (d *Deployer) tryHook(ctx context.Context, containerID string, log zerolog.Logger, name string, detectCmd, migrateCmd []string) bool {
	detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if tool exists in container
	_, err := d.docker.ContainerExec(detectCtx, containerID, detectCmd)
	if err != nil {
		return false // tool not present
	}

	// Tool detected — run migration
	log.Info().Str("tool", name).Msg("migration tool detected, running migrations")

	migrateCtx, migrateCancel := context.WithTimeout(ctx, 120*time.Second)
	defer migrateCancel()

	output, err := d.docker.ContainerExec(migrateCtx, containerID, migrateCmd)
	if err != nil {
		log.Warn().Err(err).Str("tool", name).Str("output", output).Msg("migration failed")
	} else if output != "" {
		log.Info().Str("tool", name).Str("output", output).Msg("migration completed")
	}

	return true // tool was detected, don't try others
}

func (d *Deployer) cloneRepo(ctx context.Context, app *model.App, destDir string) error {
	log := logger.With("deployer")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}

	// Get user's GitHub token for private repos
	user, err := d.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	token := user.GitHubToken
	if token != "" {
		// Try to decrypt
		if decrypted, err := crypto.Decrypt(token, d.encryptionKey); err == nil {
			token = decrypted
		}
	}

	// Build clone URL with token for authentication
	cloneURL := app.RepoURL
	maskedURL := app.RepoURL
	if token != "" {
		cloneURL = injectTokenInURL(app.RepoURL, token)
		if len(token) >= 4 {
			maskedURL = injectTokenInURL(app.RepoURL, "****"+token[len(token)-4:])
		} else {
			maskedURL = injectTokenInURL(app.RepoURL, "****")
		}
	}

	log.Debug().Str("clone_url", maskedURL).Str("branch", app.RepoBranch).Str("dest_dir", destDir).Msg("cloning repo")

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", app.RepoBranch, cloneURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Str("output", string(output)).Msg("git clone failed")
		return fmt.Errorf("git clone failed: %s", string(output))
	}

	log.Info().Str("repo", app.RepoURL).Str("branch", app.RepoBranch).Msg("repo cloned")
	return nil
}

func (d *Deployer) failDeploy(ctx context.Context, deployment *model.Deployment, app *model.App, reason string, start time.Time) {
	log := logger.With("deployer")
	duration := int(time.Since(start).Milliseconds())

	log.Debug().Str("deploy_id", deployment.ID.String()).Int("duration_ms", duration).Msg("marking deployment as failed")

	_ = d.deployRepo.UpdateStatus(ctx, deployment.ID, model.DeployFailed, reason, duration)
	_ = d.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID)

	log.Error().
		Str("app", app.Subdomain).
		Str("reason", reason).
		Int("duration_ms", duration).
		Msg("deploy failed")
}

// injectTokenInURL adds a token to a GitHub HTTPS URL for authentication.
func injectTokenInURL(repoURL, token string) string {
	// https://github.com/user/repo.git -> https://token@github.com/user/repo.git
	if len(repoURL) > 8 && repoURL[:8] == "https://" {
		return "https://" + token + "@" + repoURL[8:]
	}
	return repoURL
}

// stripDockerHeaders removes Docker multiplexed stream headers (8-byte framing)
// from container log output, returning clean text lines.
func stripDockerHeaders(data []byte) string {
	var result []byte
	i := 0
	for i < len(data) {
		// Docker header: [stream_type(1), 0(3), size_be32(4)]
		if i+8 <= len(data) && (data[i] == 1 || data[i] == 2) && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
			size := int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
			i += 8
			end := i + size
			if end > len(data) {
				end = len(data)
			}
			// Extract the payload, skip the timestamp prefix (e.g. "2026-03-08T04:04:38.649Z ")
			chunk := data[i:end]
			for len(chunk) > 30 && chunk[0] >= '0' && chunk[0] <= '9' {
				// Skip "YYYY-MM-DDThh:mm:ss.nnnnnnnnnZ " timestamp
				spaceIdx := -1
				for j := 0; j < min(35, len(chunk)); j++ {
					if chunk[j] == ' ' {
						spaceIdx = j
						break
					}
				}
				if spaceIdx > 0 && spaceIdx < 35 {
					chunk = chunk[spaceIdx+1:]
				}
				break
			}
			result = append(result, chunk...)
			i = end
		} else {
			// Not a Docker header — append raw byte
			result = append(result, data[i])
			i++
		}
	}
	return string(result)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
