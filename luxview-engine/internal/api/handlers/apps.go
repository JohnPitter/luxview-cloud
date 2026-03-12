package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

var subdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type AppHandler struct {
	appRepo       *repository.AppRepo
	userRepo      *repository.UserRepo
	serviceRepo   *repository.ServiceRepo
	container     *service.ContainerManager
	provisioner   *service.Provisioner
	github        *service.GitHubClient
	buildQueue    chan<- service.DeployRequest
	encryptionKey []byte
	auditSvc      *service.AuditService
	webhookURL    string
	webhookSecret string
}

func NewAppHandler(
	appRepo *repository.AppRepo,
	userRepo *repository.UserRepo,
	serviceRepo *repository.ServiceRepo,
	container *service.ContainerManager,
	provisioner *service.Provisioner,
	buildQueue chan<- service.DeployRequest,
	encryptionKey []byte,
	auditSvc *service.AuditService,
	webhookURL string,
	webhookSecret string,
) *AppHandler {
	return &AppHandler{
		appRepo:       appRepo,
		userRepo:      userRepo,
		serviceRepo:   serviceRepo,
		container:     container,
		provisioner:   provisioner,
		github:        service.NewGitHubClient(),
		buildQueue:    buildQueue,
		encryptionKey: encryptionKey,
		auditSvc:      auditSvc,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
	}
}

// Create creates a new app.
func (h *AppHandler) Create(w http.ResponseWriter, r *http.Request) {
	log := logger.With("apps")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	var req model.CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if req.Name == "" || req.Subdomain == "" || req.RepoURL == "" {
		writeError(w, http.StatusBadRequest, "name, subdomain, and repo_url are required")
		return
	}

	subdomain := strings.ToLower(req.Subdomain)
	if !subdomainRegex.MatchString(subdomain) {
		writeError(w, http.StatusBadRequest, "invalid subdomain format (lowercase alphanumeric and hyphens only)")
		return
	}

	if model.ReservedSubdomains[subdomain] {
		writeError(w, http.StatusConflict, "subdomain is reserved")
		return
	}

	// Check uniqueness
	existing, _ := h.appRepo.FindBySubdomain(ctx, subdomain)
	if existing != nil {
		writeError(w, http.StatusConflict, "subdomain already taken")
		return
	}

	// Plan enforcement: check max apps
	user := middleware.GetUser(ctx)
	if user.Plan != nil {
		apps, _, _ := h.appRepo.ListByUserID(ctx, userID, 1000, 0)
		if len(apps) >= user.Plan.MaxApps {
			writeError(w, http.StatusForbidden, fmt.Sprintf("Plan limit reached: your %s plan allows max %d apps", user.Plan.Name, user.Plan.MaxApps))
			return
		}
	}

	branch := req.RepoBranch
	if branch == "" {
		branch = "main"
	}

	autoDeploy := true
	if req.AutoDeploy != nil {
		autoDeploy = *req.AutoDeploy
	}

	// Encrypt env vars
	var envVarsEncrypted json.RawMessage
	if len(req.EnvVars) > 0 {
		envJSON, _ := json.Marshal(req.EnvVars)
		encrypted, err := crypto.Encrypt(string(envJSON), h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt env vars")
			return
		}
		envVarsEncrypted = json.RawMessage(`"` + encrypted + `"`)
	} else {
		envVarsEncrypted = json.RawMessage(`"{}"`)
	}

	app := &model.App{
		UserID:     userID,
		Name:       req.Name,
		Subdomain:  subdomain,
		RepoURL:    req.RepoURL,
		RepoBranch: branch,
		Status:     model.AppStatusStopped,
		EnvVars:    envVarsEncrypted,
		ResourceLimits: model.ResourceLimits{
			CPU:    "0.5",
			Memory: "512m",
			Disk:   "1g",
		},
		AutoDeploy: autoDeploy,
	}

	if err := h.appRepo.Create(ctx, app); err != nil {
		log.Error().Err(err).Msg("failed to create app")
		writeError(w, http.StatusInternalServerError, "failed to create app")
		return
	}

	// Auto-deploy: queue a deploy immediately after creation
	if autoDeploy {
		token := user.GitHubToken
		if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
			token = decrypted
		}
		owner, repo := parseRepoURL(app.RepoURL)
		commitSHA, commitMsg, err := h.github.GetLatestCommit(ctx, token, owner, repo, app.RepoBranch)
		if err != nil {
			commitSHA = "initial"
			commitMsg = "initial deploy"
		}
		select {
		case h.buildQueue <- service.DeployRequest{
			AppID:     app.ID,
			UserID:    userID,
			CommitSHA: commitSHA,
			CommitMsg: commitMsg,
		}:
			log.Info().Str("app", app.Subdomain).Msg("auto-deploy queued on creation")
		default:
			log.Warn().Str("app", app.Subdomain).Msg("build queue full, auto-deploy skipped")
		}

		// Create GitHub webhook for auto-deploy
		if owner != "" && repo != "" {
			hookID, whErr := h.github.CreateWebhook(ctx, token, owner, repo, h.webhookURL, h.webhookSecret)
			if whErr != nil {
				log.Warn().Err(whErr).Str("app", app.Subdomain).Msg("failed to create GitHub webhook on app creation")
			} else {
				app.WebhookID = &hookID
				_ = h.appRepo.Update(ctx, app)
				log.Info().Int64("hook_id", hookID).Str("app", app.Subdomain).Msg("GitHub webhook created on app creation")
			}
		}
	}

	app.EnvVarsPlain = req.EnvVars
	log.Info().Str("app", app.Subdomain).Str("user", userID.String()).Msg("app created")

	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "create",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		NewValues:    map[string]string{"name": app.Name, "subdomain": app.Subdomain, "repo_url": app.RepoURL, "branch": app.RepoBranch},
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusCreated, app)
}

// List lists all apps for the current user.
func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	apps, total, err := h.appRepo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"apps":  apps,
		"total": total,
	})
}

// Get returns a single app by ID.
func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	// Decrypt env vars for response
	if len(app.EnvVars) > 0 {
		var encrypted string
		if err := json.Unmarshal(app.EnvVars, &encrypted); err == nil {
			if decrypted, err := crypto.Decrypt(encrypted, h.encryptionKey); err == nil {
				_ = json.Unmarshal([]byte(decrypted), &app.EnvVarsPlain)
			}
		}
	}

	writeJSON(w, http.StatusOK, app)
}

// Update updates an app.
func (h *AppHandler) Update(w http.ResponseWriter, r *http.Request) {
	log := logger.With("apps")
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

	var req model.UpdateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Capture old values for audit and webhook management
	oldAutoDeploy := app.AutoDeploy
	oldValues := map[string]interface{}{
		"name":       app.Name,
		"branch":     app.RepoBranch,
		"autoDeploy": app.AutoDeploy,
		"cpu":        app.ResourceLimits.CPU,
		"memory":     app.ResourceLimits.Memory,
		"disk":       app.ResourceLimits.Disk,
	}

	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.RepoBranch != nil {
		app.RepoBranch = *req.RepoBranch
	}
	if req.AutoDeploy != nil && *req.AutoDeploy {
		user := middleware.GetUser(ctx)
		if user.Plan != nil && !user.Plan.AutoDeployEnabled {
			writeError(w, http.StatusForbidden, fmt.Sprintf("Auto-deploy is not available on your %s plan", user.Plan.Name))
			return
		}
	}
	if req.AutoDeploy != nil {
		app.AutoDeploy = *req.AutoDeploy
	}
	if req.ResourceLimits != nil {
		user := middleware.GetUser(ctx)
		if user.Plan != nil {
			if req.ResourceLimits.CPU != "" {
				reqCPU, _ := strconv.ParseFloat(req.ResourceLimits.CPU, 64)
				if reqCPU > user.Plan.MaxCPUPerApp {
					writeError(w, http.StatusForbidden, fmt.Sprintf("CPU limit %s exceeds your %s plan maximum of %.2f", req.ResourceLimits.CPU, user.Plan.Name, user.Plan.MaxCPUPerApp))
					return
				}
			}
			if req.ResourceLimits.Memory != "" {
				reqMem := parseMemoryString(req.ResourceLimits.Memory)
				planMem := parseMemoryString(user.Plan.MaxMemoryPerApp)
				if reqMem > planMem {
					writeError(w, http.StatusForbidden, fmt.Sprintf("Memory limit %s exceeds your %s plan maximum of %s", req.ResourceLimits.Memory, user.Plan.Name, user.Plan.MaxMemoryPerApp))
					return
				}
			}
			if req.ResourceLimits.Disk != "" {
				reqDisk := parseMemoryString(req.ResourceLimits.Disk)
				planDisk := parseMemoryString(user.Plan.MaxDiskPerApp)
				if reqDisk > planDisk {
					writeError(w, http.StatusForbidden, fmt.Sprintf("Disk limit %s exceeds your %s plan maximum of %s", req.ResourceLimits.Disk, user.Plan.Name, user.Plan.MaxDiskPerApp))
					return
				}
			}
		}
		app.ResourceLimits = *req.ResourceLimits
	}
	if len(req.EnvVars) > 0 {
		envJSON, _ := json.Marshal(req.EnvVars)
		encrypted, err := crypto.Encrypt(string(envJSON), h.encryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt env vars")
			return
		}
		app.EnvVars = json.RawMessage(`"` + encrypted + `"`)
	}

	if err := h.appRepo.Update(ctx, app); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update app")
		return
	}

	// Manage GitHub webhook when auto_deploy changes
	if req.AutoDeploy != nil && *req.AutoDeploy != oldAutoDeploy {
		u := middleware.GetUser(ctx)
		token := u.GitHubToken
		if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
			token = decrypted
		}
		owner, repoName := parseRepoURL(app.RepoURL)
		if owner != "" && repoName != "" {
			if app.AutoDeploy {
				hookID, err := h.github.CreateWebhook(ctx, token, owner, repoName, h.webhookURL, h.webhookSecret)
				if err != nil {
					log.Warn().Err(err).Str("app", app.Subdomain).Msg("failed to create GitHub webhook")
				} else {
					app.WebhookID = &hookID
					_ = h.appRepo.Update(ctx, app)
					log.Info().Int64("hook_id", hookID).Str("app", app.Subdomain).Msg("GitHub webhook created")
				}
			} else if app.WebhookID != nil {
				if err := h.github.DeleteWebhook(ctx, token, owner, repoName, *app.WebhookID); err != nil {
					log.Warn().Err(err).Str("app", app.Subdomain).Msg("failed to delete GitHub webhook")
				} else {
					log.Info().Int64("hook_id", *app.WebhookID).Str("app", app.Subdomain).Msg("GitHub webhook deleted")
				}
				app.WebhookID = nil
				_ = h.appRepo.Update(ctx, app)
			}
		}
	}

	user := middleware.GetUser(ctx)
	newValues := map[string]interface{}{
		"name":       app.Name,
		"branch":     app.RepoBranch,
		"autoDeploy": app.AutoDeploy,
		"cpu":        app.ResourceLimits.CPU,
		"memory":     app.ResourceLimits.Memory,
		"disk":       app.ResourceLimits.Disk,
	}
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		OldValues:    oldValues,
		NewValues:    newValues,
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, app)
}

// Delete deletes an app and its container.
func (h *AppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	log := logger.With("apps")
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

	// Stop and remove container
	if app.ContainerID != "" {
		_ = h.container.Stop(ctx, app.ContainerID)
		_ = h.container.Remove(ctx, app.ContainerID)
	}

	// Deprovision all associated services (databases, buckets, etc.)
	services, err := h.serviceRepo.ListByAppID(ctx, appID)
	if err == nil {
		for i := range services {
			if depErr := h.provisioner.Deprovision(ctx, &services[i]); depErr != nil {
				log.Warn().Err(depErr).Str("service_id", services[i].ID.String()).Msg("failed to deprovision service during app deletion")
			}
		}
	}

	// Remove GitHub webhook if exists
	if app.WebhookID != nil {
		delUser := middleware.GetUser(ctx)
		token := delUser.GitHubToken
		if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
			token = decrypted
		}
		owner, repoName := parseRepoURL(app.RepoURL)
		if owner != "" && repoName != "" {
			if err := h.github.DeleteWebhook(ctx, token, owner, repoName, *app.WebhookID); err != nil {
				log.Warn().Err(err).Str("app", app.Subdomain).Msg("failed to delete GitHub webhook during app deletion")
			} else {
				log.Info().Int64("hook_id", *app.WebhookID).Str("app", app.Subdomain).Msg("GitHub webhook deleted on app deletion")
			}
		}
	}

	if err := h.appRepo.Delete(ctx, appID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "delete",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		OldValues:    map[string]string{"name": app.Name, "subdomain": app.Subdomain},
		IPAddress:    clientIP(r),
	})

	log.Info().Str("app", app.Subdomain).Msg("app deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "app deleted"})
}

// Deploy triggers a new deployment for the app.
func (h *AppHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	log := logger.With("apps")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	user := middleware.GetUser(ctx)

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

	// Get latest commit from GitHub
	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
		token = decrypted
	}

	// Parse owner/repo from URL
	owner, repo := parseRepoURL(app.RepoURL)
	commitSHA, commitMsg, err := h.github.GetLatestCommit(ctx, token, owner, repo, app.RepoBranch)
	if err != nil {
		log.Warn().Err(err).Msg("failed to get latest commit, using placeholder")
		commitSHA = "manual"
		commitMsg = "manual deploy"
	}

	deployReq := service.DeployRequest{
		AppID:     app.ID,
		UserID:    userID,
		CommitSHA: commitSHA,
		CommitMsg: commitMsg,
		Source:    "manual",
	}

	select {
	case h.buildQueue <- deployReq:
		log.Info().Str("app", app.Subdomain).Msg("deploy queued")
		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:      user.ID,
			ActorUsername: user.Username,
			Action:       "deploy",
			ResourceType: "app",
			ResourceID:   app.ID.String(),
			ResourceName: app.Subdomain,
			NewValues:    map[string]string{"branch": app.RepoBranch},
			IPAddress:    clientIP(r),
		})
		writeJSON(w, http.StatusAccepted, map[string]string{
			"message":    "deploy queued",
			"commit_sha": commitSHA,
		})
	default:
		writeError(w, http.StatusServiceUnavailable, "build queue is full, try again later")
	}
}

// Restart restarts the app's container.
func (h *AppHandler) Restart(w http.ResponseWriter, r *http.Request) {
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

	if err := h.container.Restart(ctx, app.ContainerID); err != nil {
		_ = h.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusError, app.ContainerID)
		writeError(w, http.StatusInternalServerError, "failed to restart container")
		return
	}

	_ = h.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusRunning, app.ContainerID)

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "restart",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "app restarted"})
}

// Stop stops the app's container.
func (h *AppHandler) Stop(w http.ResponseWriter, r *http.Request) {
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

	if err := h.container.Stop(ctx, app.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stop container")
		return
	}

	_ = h.appRepo.UpdateStatus(ctx, app.ID, model.AppStatusStopped, app.ContainerID)

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "stop",
		ResourceType: "app",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "app stopped"})
}

// CheckSubdomain checks if a subdomain is available.
func (h *AppHandler) CheckSubdomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subdomain := strings.ToLower(chi.URLParam(r, "subdomain"))

	if !subdomainRegex.MatchString(subdomain) {
		writeJSON(w, http.StatusOK, map[string]bool{"available": false})
		return
	}

	if model.ReservedSubdomains[subdomain] {
		writeJSON(w, http.StatusOK, map[string]bool{"available": false})
		return
	}

	existing, _ := h.appRepo.FindBySubdomain(ctx, subdomain)
	writeJSON(w, http.StatusOK, map[string]bool{"available": existing == nil})
}

// ListGitHubRepos lists the user's GitHub repositories.
func (h *AppHandler) ListGitHubRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
		token = decrypted
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	repos, err := h.github.ListRepos(ctx, token, page, 30)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list repos from GitHub")
		return
	}

	writeJSON(w, http.StatusOK, repos)
}

// ListGitHubBranches lists branches for a repository.
func (h *AppHandler) ListGitHubBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)

	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptionKey); err == nil {
		token = decrypted
	}

	owner := chi.URLParam(r, "owner")
	repo := chi.URLParam(r, "repo")

	branches, err := h.github.ListBranches(ctx, token, owner, repo)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to list branches from GitHub")
		return
	}

	writeJSON(w, http.StatusOK, branches)
}

// ContainerLogs returns the runtime logs for an app's container.
func (h *AppHandler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
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

	if app.ContainerID == "" {
		writeJSON(w, http.StatusOK, map[string]string{"logs": ""})
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}

	reader, err := h.container.Logs(ctx, app.ContainerID, tail)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"logs": "Container not available"})
		return
	}
	defer reader.Close()

	logBytes, _ := io.ReadAll(reader)

	// Strip Docker multiplexed stream headers (8-byte prefix per frame)
	cleaned := stripDockerLogHeaders(logBytes)

	writeJSON(w, http.StatusOK, map[string]string{"logs": string(cleaned)})
}

// ContainerLogsStream streams runtime logs in real time via Server-Sent Events (SSE).
func (h *AppHandler) ContainerLogsStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	log := logger.With("logs-stream")

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

	if app.ContainerID == "" {
		writeError(w, http.StatusBadRequest, "no container running")
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	since := r.URL.Query().Get("since")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable Nginx/Traefik buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	reader, err := h.container.LogsFollow(ctx, app.ContainerID, tail, since)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\":\"container not available\"}\n\n")
		flusher.Flush()
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		// Strip Docker multiplexed stream header if present (8-byte prefix)
		cleaned := stripDockerLogHeaderLine(line)
		if len(cleaned) == 0 {
			continue
		}

		// Send as SSE data line
		fmt.Fprintf(w, "data: %s\n\n", cleaned)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Debug().Err(err).Msg("log stream ended")
	}
}

// stripDockerLogHeaderLine strips the 8-byte Docker multiplexed stream header from a single log line.
func stripDockerLogHeaderLine(line []byte) []byte {
	if len(line) >= 8 {
		// Check if this looks like a Docker header: first byte is 0, 1, or 2 (stdout/stderr/stdin)
		streamType := line[0]
		if (streamType == 0 || streamType == 1 || streamType == 2) && line[1] == 0 && line[2] == 0 && line[3] == 0 {
			size := int(line[4])<<24 | int(line[5])<<16 | int(line[6])<<8 | int(line[7])
			rest := line[8:]
			if size <= len(rest)+1 { // +1 tolerance for newline stripping
				return rest
			}
		}
	}
	return line
}

// stripDockerLogHeaders removes the 8-byte Docker multiplexed stream header from each log frame.
func stripDockerLogHeaders(data []byte) []byte {
	var result []byte
	for len(data) >= 8 {
		// Header: [stream_type(1)][0][0][0][size(4 big-endian)]
		size := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		data = data[8:]
		if size > len(data) {
			size = len(data)
		}
		result = append(result, data[:size]...)
		data = data[size:]
	}
	if len(result) == 0 {
		return data // fallback: return raw if no headers detected
	}
	return result
}

func parseRepoURL(repoURL string) (owner, repo string) {
	// https://github.com/owner/repo.git or https://github.com/owner/repo
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}
