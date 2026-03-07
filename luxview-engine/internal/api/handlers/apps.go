package handlers

import (
	"encoding/json"
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
	container     *service.ContainerManager
	github        *service.GitHubClient
	buildQueue    chan<- service.DeployRequest
	encryptionKey []byte
}

func NewAppHandler(
	appRepo *repository.AppRepo,
	userRepo *repository.UserRepo,
	container *service.ContainerManager,
	buildQueue chan<- service.DeployRequest,
	encryptionKey []byte,
) *AppHandler {
	return &AppHandler{
		appRepo:       appRepo,
		userRepo:      userRepo,
		container:     container,
		github:        service.NewGitHubClient(),
		buildQueue:    buildQueue,
		encryptionKey: encryptionKey,
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
		user := middleware.GetUser(ctx)
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
	}

	app.EnvVarsPlain = req.EnvVars
	log.Info().Str("app", app.Subdomain).Str("user", userID.String()).Msg("app created")
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

	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.RepoBranch != nil {
		app.RepoBranch = *req.RepoBranch
	}
	if req.AutoDeploy != nil {
		app.AutoDeploy = *req.AutoDeploy
	}
	if req.ResourceLimits != nil {
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

	if err := h.appRepo.Delete(ctx, appID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}

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
	}

	select {
	case h.buildQueue <- deployReq:
		log.Info().Str("app", app.Subdomain).Msg("deploy queued")
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

func parseRepoURL(repoURL string) (owner, repo string) {
	// https://github.com/owner/repo.git or https://github.com/owner/repo
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}
