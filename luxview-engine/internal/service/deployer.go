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

	// Create deployment record
	deployment := &model.Deployment{
		AppID:         app.ID,
		CommitSHA:     req.CommitSHA,
		CommitMessage: req.CommitMsg,
		Status:        model.DeployBuilding,
		ImageTag:      fmt.Sprintf("luxview/%s:%s", app.Subdomain, req.CommitSHA[:min(7, len(req.CommitSHA))]),
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
	services, err := d.serviceRepo.ListByAppID(ctx, app.ID)
	if err == nil {
		for _, svc := range services {
			var encSvc string
			if err := json.Unmarshal(svc.Credentials, &encSvc); err == nil {
				if decrypted, err := crypto.Decrypt(encSvc, d.encryptionKey); err == nil {
					var creds map[string]string
					if err := json.Unmarshal([]byte(decrypted), &creds); err == nil {
						for k, v := range d.provisioner.GetEnvVarsForService(&svc, creds) {
							envVars[k] = v
						}
					}
				}
			}
		}
		if len(services) > 0 {
			log.Info().Int("count", len(services)).Msg("injected service env vars")
		}
	}

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
		d.failDeploy(ctx, deployment, app, "container start failed: "+err.Error(), start)
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
				failReason = fmt.Sprintf("health check failed — container logs:\n%s", string(logBytes))
			}
		}

		// Rollback: stop new, restart old
		_ = d.container.Stop(ctx, containerID)
		_ = d.container.Remove(ctx, containerID)
		if oldContainerID != "" {
			_ = d.container.Restart(ctx, oldContainerID)
			_ = d.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, oldContainerID)
		}
		d.failDeploy(ctx, deployment, app, failReason, start)
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
// For example, runs Prisma migrations if the app uses Prisma.
func (d *Deployer) runPostDeployHooks(ctx context.Context, containerID string, stack string) {
	log := logger.With("deployer")

	// Check if container has prisma by attempting to run prisma db push
	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Try prisma db push (works for both migration-based and push-based projects)
	output, err := d.docker.ContainerExec(execCtx, containerID, []string{"npx", "prisma", "db", "push", "--skip-generate"})
	if err != nil {
		// Not a Prisma project or prisma not installed — that's fine, skip silently
		log.Debug().Err(err).Msg("prisma db push skipped (not a prisma project or failed)")
		return
	}

	if output != "" {
		log.Info().Str("output", output).Msg("prisma db push completed")
	}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
