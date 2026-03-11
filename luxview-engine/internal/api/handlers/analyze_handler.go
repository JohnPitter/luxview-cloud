package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/agent"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/detector"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

type AnalyzeHandler struct {
	appRepo      *repository.AppRepo
	userRepo     *repository.UserRepo
	deployRepo   *repository.DeploymentRepo
	settingsRepo *repository.SettingsRepo
	serviceRepo  *repository.ServiceRepo
	provisioner  *service.Provisioner
	agent        *agent.DeployAgent
	encryptKey   []byte
	auditSvc     *service.AuditService
}

func NewAnalyzeHandler(
	appRepo *repository.AppRepo,
	userRepo *repository.UserRepo,
	deployRepo *repository.DeploymentRepo,
	settingsRepo *repository.SettingsRepo,
	serviceRepo *repository.ServiceRepo,
	provisioner *service.Provisioner,
	encryptKey []byte,
	auditSvc *service.AuditService,
) *AnalyzeHandler {
	return &AnalyzeHandler{
		appRepo:      appRepo,
		userRepo:     userRepo,
		deployRepo:   deployRepo,
		settingsRepo: settingsRepo,
		serviceRepo:  serviceRepo,
		provisioner:  provisioner,
		agent:        agent.NewDeployAgent(),
		encryptKey:   encryptKey,
		auditSvc:     auditSvc,
	}
}

// aiConfig holds the resolved AI configuration from platform settings.
type aiConfig struct {
	apiKey string
	model  string
}

// getAIConfig reads and validates AI settings from the settings repo.
func (h *AnalyzeHandler) getAIConfig(ctx context.Context) (*aiConfig, error) {
	settings, err := h.settingsRepo.GetAll(ctx, "ai_")
	if err != nil {
		return nil, fmt.Errorf("get AI settings: %w", err)
	}

	if settings["enabled"] != "true" {
		return nil, fmt.Errorf("AI features are disabled")
	}

	apiKey := settings["api_key"]
	if apiKey == "" {
		return nil, fmt.Errorf("OpenRouter API key not configured")
	}

	model := settings["model"]
	if model == "" {
		model = "anthropic/claude-sonnet-4"
	}

	return &aiConfig{apiKey: apiKey, model: model}, nil
}

// cloneRepo clones the app repository to a temporary directory using the user's GitHub token.
func (h *AnalyzeHandler) cloneRepo(ctx context.Context, appID uuid.UUID, repoURL, branch string) (string, error) {
	// Get the app owner's user to access their GitHub token
	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		return "", fmt.Errorf("find app: %w", err)
	}

	user, err := h.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return "", fmt.Errorf("find user: %w", err)
	}

	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptKey); err == nil {
		token = decrypted
	}

	// Inject token into clone URL: https://TOKEN@github.com/...
	cloneURL := repoURL
	if strings.HasPrefix(cloneURL, "https://github.com/") {
		cloneURL = "https://" + token + "@" + strings.TrimPrefix(cloneURL, "https://")
	}

	destDir := filepath.Join(os.TempDir(), "luxview-analyze", appID.String())
	// Clean up any previous clone
	_ = os.RemoveAll(destDir)
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, cloneURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	return destDir, nil
}

// Analyze handles POST /apps/{id}/analyze — runs AI analysis on the app's repository.
// Falls back to deterministic detection if AI is unavailable.
func (h *AnalyzeHandler) Analyze(w http.ResponseWriter, r *http.Request) {
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

	cfg, aiErr := h.getAIConfig(ctx)
	aiEnabled := aiErr == nil

	log.Debug().Str("app_id", appID.String()).Bool("ai_enabled", aiEnabled).Msg("starting analysis")

	lang := r.Header.Get("Accept-Language")
	if lang == "" {
		lang = "en"
	}

	cloneDir, err := h.cloneRepo(ctx, appID, app.RepoURL, app.RepoBranch)
	if err != nil {
		log.Error().Err(err).Msg("failed to clone repo")
		writeError(w, http.StatusInternalServerError, "failed to clone repository")
		return
	}
	defer os.RemoveAll(cloneDir)

	log.Debug().Str("clone_dir", cloneDir).Msg("repo cloned for analysis")

	if aiErr != nil {
		// AI disabled — use deterministic detection
		log.Info().Msg("AI unavailable, using deterministic analysis")
		result := detector.Analyze(cloneDir)
		if result.RequiresAI {
			log.Warn().Msg("monorepo detected but AI is disabled — returning requiresAi flag")
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	// AI is available — check if it's a monorepo that needs AI
	// (deterministic detection can't handle workspace:* dependencies)
	deterministicResult := detector.Analyze(cloneDir)
	if deterministicResult.RequiresAI {
		log.Info().Msg("monorepo detected, forcing AI analysis for Dockerfile generation")
	}

	log.Debug().Str("model", cfg.model).Msg("running AI analysis")

	// AI enabled — existing flow
	result, err := h.agent.Analyze(ctx, cfg.apiKey, cfg.model, cloneDir, lang)
	if err != nil {
		log.Error().Err(err).Str("app", app.Subdomain).Msg("analysis failed")
		writeError(w, http.StatusInternalServerError, "analysis failed: "+err.Error())
		return
	}

	dockerfileLen := len(result.Dockerfile)
	log.Debug().
		Str("app", app.Subdomain).
		Str("stack", result.Stack).
		Int("port", result.Port).
		Int("suggestions", len(result.Suggestions)).
		Int("service_recommendations", len(result.ServiceRecommendations)).
		Int("dockerfile_len", dockerfileLen).
		Msg("analysis result")

	log.Info().Str("app", app.Subdomain).Str("stack", result.Stack).Msg("analysis complete")
	writeJSON(w, http.StatusOK, result)
}

// AnalyzeFailure handles POST /apps/{id}/analyze-failure — diagnoses a failed build.
func (h *AnalyzeHandler) AnalyzeFailure(w http.ResponseWriter, r *http.Request) {
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

	cfg, err := h.getAIConfig(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("AI config unavailable")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get latest deployment's build log
	deployments, _, err := h.deployRepo.ListByAppID(ctx, appID, 1, 0)
	if err != nil || len(deployments) == 0 {
		writeError(w, http.StatusNotFound, "no deployments found")
		return
	}

	// ListByAppID doesn't include build_log, so fetch the full deployment
	latestDeploy, err := h.deployRepo.FindByID(ctx, deployments[0].ID)
	if err != nil || latestDeploy == nil {
		writeError(w, http.StatusInternalServerError, "failed to get deployment details")
		return
	}

	lang := r.Header.Get("Accept-Language")
	if lang == "" {
		lang = "en"
	}

	var dockerfile string
	if app.CustomDockerfile != nil {
		dockerfile = *app.CustomDockerfile
	}

	log.Debug().Str("deploy_id", latestDeploy.ID.String()).Int("build_log_len", len(latestDeploy.BuildLog)).Msg("analyzing failure")

	cloneDir, err := h.cloneRepo(ctx, appID, app.RepoURL, app.RepoBranch)
	if err != nil {
		log.Error().Err(err).Msg("failed to clone repo")
		writeError(w, http.StatusInternalServerError, "failed to clone repository")
		return
	}
	defer os.RemoveAll(cloneDir)

	result, err := h.agent.AnalyzeFailure(ctx, cfg.apiKey, cfg.model, cloneDir, latestDeploy.BuildLog, dockerfile, lang)
	if err != nil {
		log.Error().Err(err).Str("app", app.Subdomain).Msg("failure analysis failed")
		writeError(w, http.StatusInternalServerError, "failure analysis failed: "+err.Error())
		return
	}

	log.Info().Str("app", app.Subdomain).Str("diagnosis", result.Diagnosis).Msg("failure analysis complete")
	writeJSON(w, http.StatusOK, result)
}

// SaveDockerfile handles PUT /apps/{id}/dockerfile — saves a custom Dockerfile.
func (h *AnalyzeHandler) SaveDockerfile(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.appRepo.UpdateCustomDockerfile(ctx, appID, &body.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save dockerfile")
		return
	}

	log := logger.With("analyze")
	log.Info().Str("app", app.Subdomain).Msg("custom dockerfile saved")
	writeJSON(w, http.StatusOK, map[string]string{"message": "dockerfile saved"})
}

// DeleteDockerfile handles DELETE /apps/{id}/dockerfile — removes the custom Dockerfile.
func (h *AnalyzeHandler) DeleteDockerfile(w http.ResponseWriter, r *http.Request) {
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

	if err := h.appRepo.UpdateCustomDockerfile(ctx, appID, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete dockerfile")
		return
	}

	log := logger.With("analyze")
	log.Info().Str("app", app.Subdomain).Msg("custom dockerfile deleted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "dockerfile deleted"})
}

type applyAnalysisRequest struct {
	Dockerfile string   `json:"dockerfile"`
	Services   []string `json:"services"`
}

type applyAnalysisResponse struct {
	Message string `json:"message"`
}

// ApplyAnalysis handles POST /apps/{id}/apply-analysis — saves the Dockerfile and provisions services.
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

	log.Debug().
		Str("app_id", appID.String()).
		Int("dockerfile_len", len(req.Dockerfile)).
		Int("services_count", len(req.Services)).
		Msg("applying analysis")

	// Save Dockerfile
	if req.Dockerfile != "" {
		if err := h.appRepo.UpdateCustomDockerfile(ctx, appID, &req.Dockerfile); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save dockerfile")
			return
		}
		log.Info().Str("app", app.Subdomain).Msg("dockerfile saved via apply-analysis")
	}

	// Provision selected services
	for _, svcType := range req.Services {
		serviceType := model.ServiceType(svcType)
		svc, provErr := h.provisioner.Provision(ctx, appID, serviceType)
		if provErr != nil {
			if strings.Contains(provErr.Error(), "already provisioned") {
				existing, findErr := h.serviceRepo.FindByAppAndType(ctx, appID, serviceType)
				if findErr != nil || existing == nil {
					log.Warn().Str("service", svcType).Err(provErr).Msg("failed to find existing service")
					continue
				}
				svc = existing
			} else {
				log.Error().Str("service", svcType).Err(provErr).Msg("failed to provision service")
				continue
			}
		}
		log.Info().Str("service", svcType).Str("id", svc.ID.String()).Msg("service provisioned via apply-analysis")
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "create",
		ResourceType: "deployment",
		ResourceID:   app.ID.String(),
		ResourceName: app.Subdomain,
		NewValues:    map[string]interface{}{"services": req.Services},
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, applyAnalysisResponse{
		Message: "Analysis applied successfully",
	})
}
