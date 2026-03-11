package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
}

// Deployer orchestrates the full deploy flow.
type Deployer struct {
	appRepo        *repository.AppRepo
	deployRepo     *repository.DeploymentRepo
	userRepo       *repository.UserRepo
	serviceRepo    *repository.ServiceRepo
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

	app, err := d.appRepo.FindByID(ctx, req.AppID)
	if err != nil || app == nil {
		return fmt.Errorf("app not found: %w", err)
	}

	// Determine deploy source: AI-generated Dockerfile or auto-detected
	deploySource := "auto"
	if app.CustomDockerfile != nil && *app.CustomDockerfile != "" {
		deploySource = "ai"
	}

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

	// If app has a custom Dockerfile (from AI agent or user), inject it into the build dir
	if app.CustomDockerfile != nil && *app.CustomDockerfile != "" {
		dockerfilePath := filepath.Join(buildDir, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, []byte(*app.CustomDockerfile), 0644); err != nil {
			log.Warn().Err(err).Msg("failed to write custom Dockerfile, falling back to auto-detect")
		} else {
			log.Info().Str("app", app.Subdomain).Msg("using custom Dockerfile from AI agent")
		}
	}

	// Detect stack
	result := d.detector.Detect(buildDir)
	if result == nil {
		d.failDeploy(ctx, deployment, app, "no supported stack detected", start)
		return fmt.Errorf("no buildpack detected for app %s", app.Subdomain)
	}
	bp := result.Buildpack
	buildDir = result.BuildDir

	// If using a custom Dockerfile, detect the EXPOSE port
	if dfp, ok := bp.(*buildpack.DockerfilePack); ok {
		dfp.DetectPort(buildDir)
	}

	// Update app stack
	app.Stack = bp.Name()

	// Build image
	buildCtx, cancel := context.WithTimeout(ctx, d.buildTimeout)
	defer cancel()

	buildLog, err := d.builder.Build(buildCtx, buildDir, bp, deployment.ImageTag)
	if err != nil {
		d.failDeploy(ctx, deployment, app, buildLog+"\n"+err.Error(), start)
		return fmt.Errorf("build failed: %w", err)
	}

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

	// Inject service env vars (DATABASE_URL, REDIS_URL, etc.)
	// Service vars are injected first, then user env vars override them.
	// This ensures user-defined DATABASE_URL takes priority.
	serviceEnvVars := make(map[string]string)
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
					}
				}
			}
		}
		if len(services) > 0 {
			log.Info().Int("count", len(services)).Msg("injected service env vars")
		}
	}

	// Merge: service env vars first, then user env vars override
	mergedEnvVars := make(map[string]string)
	for k, v := range serviceEnvVars {
		mergedEnvVars[k] = v
	}
	for k, v := range envVars {
		mergedEnvVars[k] = v
	}
	envVars = mergedEnvVars

	// Stop old container (blue-green)
	oldContainerID := app.ContainerID
	if oldContainerID != "" {
		_ = d.container.Stop(ctx, oldContainerID)
	}

	// Start new container
	containerID, err := d.container.Start(ctx, app, deployment.ImageTag, envVars)
	if err != nil {
		// Try to restart old container on failure
		if oldContainerID != "" {
			_ = d.container.Restart(ctx, oldContainerID)
		}
		d.failDeploy(ctx, deployment, app, buildLog+"\n\n--- DEPLOY FAILED ---\ncontainer start failed: "+err.Error(), start)
		return fmt.Errorf("start container: %w", err)
	}

	// Health check — longer timeout for slow-starting stacks (Java, etc.)
	healthTimeout := 120 * time.Second
	switch bp.Name() {
	case "java", "nextjs", "dockerfile":
		healthTimeout = 180 * time.Second
	}
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

	// Remove old container
	if oldContainerID != "" {
		_ = d.container.Remove(ctx, oldContainerID)
	}

	// Run post-deploy hooks (e.g., Prisma migrations)
	d.runPostDeployHooks(ctx, containerID, bp.Name())

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
// Detects ORM/migration tools and runs the appropriate migration command.
func (d *Deployer) runPostDeployHooks(ctx context.Context, containerID string, stack string) {
	log := logger.With("deployer")

	// First, check if package.json has a db:migrate script (monorepos, custom setups)
	if d.tryMigration(ctx, containerID, log, "package.json db:migrate",
		[]string{"sh", "-c", "cat package.json 2>/dev/null | grep -q '\"db:migrate\"'"},
		[]string{"npm", "run", "db:migrate"}) {
		return
	}

	// Also check for "migrate" script
	if d.tryMigration(ctx, containerID, log, "package.json migrate",
		[]string{"sh", "-c", "cat package.json 2>/dev/null | grep -q '\"migrate\"'"},
		[]string{"npm", "run", "migrate"}) {
		return
	}

	// Prisma — push schema to DB (works without migration history)
	if d.tryMigration(ctx, containerID, log, "prisma db push",
		[]string{"sh", "-c", "test -f node_modules/.prisma/client/index.js || test -f prisma/schema.prisma"},
		[]string{"npx", "prisma", "db", "push", "--skip-generate"}) {
		return
	}

	// Drizzle — push schema to DB
	if d.tryMigration(ctx, containerID, log, "drizzle-kit push",
		[]string{"sh", "-c", "test -f node_modules/drizzle-kit/bin.cjs"},
		[]string{"npx", "drizzle-kit", "push"}) {
		return
	}

	// TypeORM — run migrations
	if d.tryMigration(ctx, containerID, log, "typeorm migrations",
		[]string{"sh", "-c", "test -f node_modules/typeorm/cli.js"},
		[]string{"npx", "typeorm", "migration:run", "-d", "dist/data-source.js"}) {
		return
	}

	// Knex — run migrations
	if d.tryMigration(ctx, containerID, log, "knex migrate",
		[]string{"sh", "-c", "test -f node_modules/.bin/knex"},
		[]string{"npx", "knex", "migrate:latest"}) {
		return
	}

	// Python — Django migrate
	if d.tryMigration(ctx, containerID, log, "django migrate",
		[]string{"sh", "-c", "test -f manage.py"},
		[]string{"python", "manage.py", "migrate", "--noinput"}) {
		return
	}

	// Python — Alembic
	if d.tryMigration(ctx, containerID, log, "alembic upgrade",
		[]string{"sh", "-c", "test -f alembic.ini"},
		[]string{"alembic", "upgrade", "head"}) {
		return
	}

	// Java — Flyway (via Spring Boot, runs automatically on startup typically)
	// Go — goose, golang-migrate (typically embedded in app binary)
	// These usually run on app startup, no need for explicit hook.

	log.Debug().Msg("no migration tool detected, skipping post-deploy hooks")
}

// tryMigration checks if a migration tool is present and runs the migration command.
// Returns true if the tool was detected (regardless of migration success/failure).
func (d *Deployer) tryMigration(ctx context.Context, containerID string, log zerolog.Logger, name string, detectCmd, migrateCmd []string) bool {
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
	if token != "" {
		cloneURL = injectTokenInURL(app.RepoURL, token)
	}

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
