package handlers

import (
	"context"
	"encoding/base64"
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
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/crypto"
	"github.com/luxview/engine/pkg/logger"
)

// AutoMigrateHandler handles automatic service migration with PR creation.
type AutoMigrateHandler struct {
	appRepo      *repository.AppRepo
	userRepo     *repository.UserRepo
	serviceRepo  *repository.ServiceRepo
	settingsRepo *repository.SettingsRepo
	provisioner  *service.Provisioner
	github       *service.GitHubClient
	agent        *agent.DeployAgent
	encryptKey   []byte
}

func NewAutoMigrateHandler(
	appRepo *repository.AppRepo,
	userRepo *repository.UserRepo,
	serviceRepo *repository.ServiceRepo,
	settingsRepo *repository.SettingsRepo,
	provisioner *service.Provisioner,
	encryptKey []byte,
) *AutoMigrateHandler {
	return &AutoMigrateHandler{
		appRepo:      appRepo,
		userRepo:     userRepo,
		serviceRepo:  serviceRepo,
		settingsRepo: settingsRepo,
		provisioner:  provisioner,
		github:       service.NewGitHubClient(),
		agent:        agent.NewDeployAgent(),
		encryptKey:   encryptKey,
	}
}

type autoMigrateRequest struct {
	ServiceType string `json:"service_type"`
}

type autoMigrateResponse struct {
	ServiceID string `json:"service_id"`
	PRURL     string `json:"pr_url,omitempty"`
	Message   string `json:"message"`
}

// AutoMigrate handles POST /apps/{id}/auto-migrate
// 1. Provisions the service
// 2. Calls AI to generate code changes
// 3. Creates a branch with changes and a PR
func (h *AutoMigrateHandler) AutoMigrate(w http.ResponseWriter, r *http.Request) {
	log := logger.With("auto-migrate")
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

	var req autoMigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	serviceType := model.ServiceType(req.ServiceType)
	switch serviceType {
	case model.ServicePostgres, model.ServiceRedis, model.ServiceMongoDB, model.ServiceRabbitMQ, model.ServiceS3:
		// valid
	default:
		writeError(w, http.StatusBadRequest, "invalid service type")
		return
	}

	// Check plan limits
	user := middleware.GetUser(ctx)
	if user.Plan != nil {
		existingServices, _ := h.serviceRepo.ListByAppID(ctx, appID)
		if len(existingServices) >= user.Plan.MaxServicesPerApp {
			writeError(w, http.StatusForbidden, fmt.Sprintf("Plan limit reached: max %d services per app", user.Plan.MaxServicesPerApp))
			return
		}
	}

	// Step 1: Provision the service (or reuse existing)
	log.Info().Str("app", app.Subdomain).Str("service", req.ServiceType).Msg("provisioning service")
	svc, err := h.provisioner.Provision(ctx, appID, serviceType)
	if err != nil {
		// If already provisioned, find existing and continue
		if strings.Contains(err.Error(), "already provisioned") {
			existing, findErr := h.serviceRepo.FindByAppAndType(ctx, appID, serviceType)
			if findErr != nil || existing == nil {
				writeError(w, http.StatusInternalServerError, "failed to find existing service")
				return
			}
			svc = existing
			log.Info().Str("service_id", svc.ID.String()).Msg("reusing existing service")
		} else {
			log.Error().Err(err).Msg("failed to provision service")
			writeError(w, http.StatusInternalServerError, "failed to provision service: "+err.Error())
			return
		}
	} else {
		log.Info().Str("service_id", svc.ID.String()).Msg("service provisioned")
	}

	// Step 2: Get AI config
	cfg, err := h.getAIConfig(ctx)
	if err != nil {
		// Service provisioned but AI unavailable — still return success
		log.Warn().Err(err).Msg("AI unavailable, skipping code changes")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. AI is not configured — code changes were not generated.",
		})
		return
	}

	// Step 3: Clone repo and generate code changes
	cloneDir, err := h.cloneRepo(ctx, app)
	if err != nil {
		log.Error().Err(err).Msg("failed to clone repo for code generation")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Failed to clone repo for code changes.",
		})
		return
	}
	defer os.RemoveAll(cloneDir)

	lang := r.Header.Get("Accept-Language")
	if lang == "" {
		lang = "en"
	}

	migration, err := h.agent.GenerateCodeChanges(ctx, cfg.apiKey, cfg.model, cloneDir, req.ServiceType, lang)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate code changes")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Code change generation failed: " + err.Error(),
		})
		return
	}

	if len(migration.CodeChanges) == 0 {
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. No code changes needed.",
		})
		return
	}

	// Step 4: Create PR via GitHub API
	prURL, err := h.createPR(ctx, app, migration, cloneDir)
	if err != nil {
		log.Error().Err(err).Msg("failed to create PR")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Failed to create PR: " + err.Error(),
		})
		return
	}

	log.Info().Str("pr_url", prURL).Msg("migration PR created")
	writeJSON(w, http.StatusCreated, autoMigrateResponse{
		ServiceID: svc.ID.String(),
		PRURL:     prURL,
		Message:   "Service provisioned and migration PR created.",
	})
}

func (h *AutoMigrateHandler) getAIConfig(ctx context.Context) (*aiConfig, error) {
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

func (h *AutoMigrateHandler) cloneRepo(ctx context.Context, app *model.App) (string, error) {
	user, err := h.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return "", fmt.Errorf("find user: %w", err)
	}

	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptKey); err == nil {
		token = decrypted
	}

	cloneURL := app.RepoURL
	if strings.HasPrefix(cloneURL, "https://github.com/") {
		cloneURL = "https://" + token + "@" + strings.TrimPrefix(cloneURL, "https://")
	}

	destDir := fmt.Sprintf("%s/luxview-migrate/%s", os.TempDir(), app.ID.String())
	_ = os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", app.RepoBranch, cloneURL, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	return destDir, nil
}

func (h *AutoMigrateHandler) createPR(ctx context.Context, app *model.App, migration *agent.MigrationResult, cloneDir string) (string, error) {
	log := logger.With("auto-migrate")
	user, err := h.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return "", fmt.Errorf("find user: %w", err)
	}

	token := user.GitHubToken
	if decrypted, err := crypto.Decrypt(token, h.encryptKey); err == nil {
		token = decrypted
	}

	// Parse owner/repo from RepoURL
	owner, repo := parseOwnerRepo(app.RepoURL)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("could not parse owner/repo from %s", app.RepoURL)
	}

	// Get the latest commit SHA from the deploy branch
	baseBranch := app.RepoBranch
	sha, _, err := h.github.GetLatestCommit(ctx, token, owner, repo, baseBranch)
	if err != nil {
		return "", fmt.Errorf("get latest commit: %w", err)
	}

	// Create a new branch
	branchName := fmt.Sprintf("luxview/migrate-%s-%s", migration.CodeChanges[0].File, app.ID.String()[:8])
	branchName = sanitizeBranchName(branchName)
	if err := h.github.CreateBranch(ctx, token, owner, repo, branchName, sha); err != nil {
		return "", fmt.Errorf("create branch: %w", err)
	}

	// Commit each file change and track if any package.json was modified
	hasPackageJSONChange := false
	for _, change := range migration.CodeChanges {
		if change.Action == "delete" {
			continue // TODO: implement file deletion via GitHub API
		}

		// Get existing file SHA if modifying
		var fileSHA string
		if change.Action == "modify" {
			_, existingSHA, err := h.github.GetFileContent(ctx, token, owner, repo, change.File, baseBranch)
			if err == nil {
				fileSHA = existingSHA
			}
		}

		content := base64.StdEncoding.EncodeToString([]byte(change.Content))
		commitMsg := fmt.Sprintf("chore(luxview): %s", change.Description)
		if err := h.github.CreateOrUpdateFile(ctx, token, owner, repo, change.File, commitMsg, content, fileSHA, branchName); err != nil {
			return "", fmt.Errorf("commit file %s: %w", change.File, err)
		}

		if strings.HasSuffix(change.File, "package.json") {
			hasPackageJSONChange = true
		}
	}

	// If package.json was modified, regenerate and commit the lockfile
	if hasPackageJSONChange {
		if err := h.updateLockfile(ctx, token, owner, repo, branchName, baseBranch, cloneDir, migration.CodeChanges); err != nil {
			log.Warn().Err(err).Msg("failed to update lockfile — PR will need manual lockfile update")
		}
	}

	// Create PR
	prTitle := migration.PRTitle
	if prTitle == "" {
		prTitle = "chore: migrate to LuxView Cloud managed service"
	}
	prBody := migration.PRBody
	if prBody == "" {
		prBody = "Automated migration by LuxView Cloud Deploy Agent."
	}
	prBody += "\n\n---\n*This PR was automatically generated by [LuxView Cloud](https://luxview.cloud).*"

	prURL, err := h.github.CreatePullRequest(ctx, token, owner, repo, prTitle, prBody, branchName, baseBranch)
	if err != nil {
		return "", fmt.Errorf("create PR: %w", err)
	}

	return prURL, nil
}

// updateLockfile applies code changes to the local clone, runs the package manager's
// lockfile update command, and commits the updated lockfile to the branch via GitHub API.
func (h *AutoMigrateHandler) updateLockfile(
	ctx context.Context, token, owner, repo, branch, baseBranch, cloneDir string,
	codeChanges []agent.CodeChange,
) error {
	log := logger.With("auto-migrate")

	// Apply code changes to local clone so lockfile generation has the updated package.json
	for _, change := range codeChanges {
		if change.Action == "delete" {
			continue
		}
		targetPath := filepath.Join(cloneDir, filepath.FromSlash(change.File))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", change.File, err)
		}
		if err := os.WriteFile(targetPath, []byte(change.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", change.File, err)
		}
	}

	// Detect package manager by existing lockfile
	type pmConfig struct {
		lockfile   string
		installCmd string
		installArg []string
	}
	managers := []pmConfig{
		{"pnpm-lock.yaml", "pnpm", []string{"install", "--lockfile-only"}},
		{"yarn.lock", "yarn", []string{"install", "--mode", "update-lockfile"}},
		{"package-lock.json", "npm", []string{"install", "--package-lock-only"}},
	}

	var pm *pmConfig
	for i, m := range managers {
		if _, err := os.Stat(filepath.Join(cloneDir, m.lockfile)); err == nil {
			pm = &managers[i]
			break
		}
	}
	if pm == nil {
		return fmt.Errorf("no lockfile found in repo")
	}

	log.Info().Str("pm", pm.installCmd).Str("lockfile", pm.lockfile).Msg("regenerating lockfile")

	// Run the lockfile update command
	cmd := exec.CommandContext(ctx, pm.installCmd, pm.installArg...)
	cmd.Dir = cloneDir
	cmd.Env = append(os.Environ(), "CI=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s install failed: %s: %w", pm.installCmd, string(output), err)
	}

	// Read the updated lockfile
	lockfilePath := filepath.Join(cloneDir, pm.lockfile)
	lockfileContent, err := os.ReadFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("read updated lockfile: %w", err)
	}

	// Get existing lockfile SHA from the branch (it may have been updated by prior commits)
	_, existingSHA, err := h.github.GetFileContent(ctx, token, owner, repo, pm.lockfile, branch)
	if err != nil {
		// Try base branch
		_, existingSHA, err = h.github.GetFileContent(ctx, token, owner, repo, pm.lockfile, baseBranch)
		if err != nil {
			return fmt.Errorf("get lockfile SHA: %w", err)
		}
	}

	// Commit the updated lockfile
	content := base64.StdEncoding.EncodeToString(lockfileContent)
	commitMsg := "chore(luxview): update lockfile after dependency changes"
	if err := h.github.CreateOrUpdateFile(ctx, token, owner, repo, pm.lockfile, commitMsg, content, existingSHA, branch); err != nil {
		return fmt.Errorf("commit lockfile: %w", err)
	}

	log.Info().Str("lockfile", pm.lockfile).Msg("lockfile updated in PR")
	return nil
}

// parseOwnerRepo extracts "owner" and "repo" from a GitHub URL.
func parseOwnerRepo(repoURL string) (string, string) {
	// Handle https://github.com/owner/repo or https://github.com/owner/repo.git
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

// sanitizeBranchName cleans a branch name for Git.
func sanitizeBranchName(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	// Re-add the luxview/ prefix
	if !strings.HasPrefix(name, "luxview/") && strings.HasPrefix(name, "luxview-") {
		name = "luxview/" + name[len("luxview-"):]
	}
	// Keep only safe chars
	var clean []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '/' || c == '.' {
			clean = append(clean, c)
		}
	}
	result := string(clean)
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}
