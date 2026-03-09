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
	"time"

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

	// Step 4: Validate and apply changes to local clone
	// Filter out destructive changes (AI sometimes replaces real code with stubs)
	allChanges := validateCodeChanges(cloneDir, migration.CodeChanges)
	if len(allChanges) == 0 {
		log.Warn().Msg("all code changes were rejected by validation")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Code changes were rejected because they would destroy existing code.",
		})
		return
	}
	if err := applyChangesToClone(cloneDir, allChanges); err != nil {
		log.Error().Err(err).Msg("failed to apply code changes to clone")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Failed to apply code changes locally: " + err.Error(),
		})
		return
	}

	buildPassed := false
	var lastBuildOutput string
	const maxBuildRetries = 3

	for attempt := 0; attempt < maxBuildRetries; attempt++ {
		log.Info().Int("attempt", attempt+1).Msg("verifying build")
		output, buildErr := verifyBuild(ctx, cloneDir)
		if buildErr == nil {
			buildPassed = true
			log.Info().Int("attempt", attempt+1).Msg("build passed")
			break
		}

		lastBuildOutput = output
		log.Warn().Int("attempt", attempt+1).Str("output", truncateString(output, 500)).Msg("build failed, asking AI for fixes")

		fixes, fixErr := h.agent.FixBuildErrors(ctx, cfg.apiKey, cfg.model, cloneDir, output, req.ServiceType, lang)
		if fixErr != nil {
			log.Error().Err(fixErr).Msg("AI failed to generate build fixes")
			break
		}
		if len(fixes) == 0 {
			log.Warn().Msg("AI returned no fixes for build errors")
			break
		}

		if err := applyChangesToClone(cloneDir, fixes); err != nil {
			log.Error().Err(err).Msg("failed to apply fix changes to clone")
			break
		}
		allChanges = append(allChanges, fixes...)
	}

	// Update migration with all changes (original + fixes) for PR creation
	migration.CodeChanges = allChanges
	if !buildPassed && lastBuildOutput != "" {
		buildNote := "\n\n---\n**⚠️ Build verification failed** — the generated changes may require manual adjustments.\n\n<details><summary>Build output</summary>\n\n```\n" + truncateString(lastBuildOutput, 2000) + "\n```\n</details>"
		migration.PRBody += buildNote
	}

	// Step 5: Create PR via GitHub API
	prURL, err := h.createPR(ctx, app, migration, cloneDir)
	if err != nil {
		log.Error().Err(err).Msg("failed to create PR")
		writeJSON(w, http.StatusCreated, autoMigrateResponse{
			ServiceID: svc.ID.String(),
			Message:   "Service provisioned. Failed to create PR: " + err.Error(),
		})
		return
	}

	message := "Service provisioned and migration PR created."
	if !buildPassed {
		message = "Service provisioned and migration PR created. Build verification failed — PR may need manual fixes."
	}

	log.Info().Str("pr_url", prURL).Bool("build_passed", buildPassed).Msg("migration PR created")
	writeJSON(w, http.StatusCreated, autoMigrateResponse{
		ServiceID: svc.ID.String(),
		PRURL:     prURL,
		Message:   message,
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

		// Get existing file SHA — check branch first (may have been modified by earlier commits), then base
		var fileSHA string
		if change.Action == "modify" || change.Action == "create" {
			// Try the PR branch first (file may have been committed in a previous iteration)
			_, existingSHA, err := h.github.GetFileContent(ctx, token, owner, repo, change.File, branchName)
			if err != nil {
				// Fallback to base branch
				_, existingSHA, err = h.github.GetFileContent(ctx, token, owner, repo, change.File, baseBranch)
				if err == nil {
					fileSHA = existingSHA
				}
			} else {
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
		if err := h.updateLockfile(ctx, token, owner, repo, branchName, baseBranch, cloneDir); err != nil {
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

// applyChangesToClone writes code changes to the local clone directory.
func applyChangesToClone(cloneDir string, changes []agent.CodeChange) error {
	for _, change := range changes {
		targetPath := filepath.Join(cloneDir, filepath.FromSlash(change.File))
		if change.Action == "delete" {
			_ = os.Remove(targetPath)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", change.File, err)
		}
		if err := os.WriteFile(targetPath, []byte(change.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", change.File, err)
		}
	}
	return nil
}

// verifyBuild detects the project's build system and runs a build command to verify
// the code compiles. Returns the combined output and any error.
func verifyBuild(ctx context.Context, cloneDir string) (string, error) {
	log := logger.With("auto-migrate")

	type buildConfig struct {
		name       string
		detectFile string
		installCmd []string
		buildCmd   []string
		lintCmd    []string // optional lint/typecheck command
		testCmd    []string // optional test command
	}

	configs := []buildConfig{
		// Node.js (pnpm/yarn/npm) — covers Node.js, Next.js, Vite stacks
		{"pnpm", "pnpm-lock.yaml", []string{"pnpm", "install", "--no-frozen-lockfile"}, []string{"pnpm", "run", "build"}, []string{"pnpm", "run", "lint"}, []string{"pnpm", "run", "test"}},
		{"yarn", "yarn.lock", []string{"yarn", "install", "--no-immutable"}, []string{"yarn", "run", "build"}, []string{"yarn", "run", "lint"}, []string{"yarn", "run", "test"}},
		{"npm", "package-lock.json", []string{"npm", "install", "--no-package-lock"}, []string{"npm", "run", "build"}, []string{"npm", "run", "lint"}, []string{"npm", "run", "test"}},
		{"npm-fallback", "package.json", []string{"npm", "install", "--no-package-lock"}, []string{"npm", "run", "build"}, []string{"npm", "run", "lint"}, []string{"npm", "run", "test"}},
		// Go
		{"go", "go.mod", nil, []string{"go", "build", "./..."}, []string{"go", "vet", "./..."}, []string{"go", "test", "./..."}},
		// Rust
		{"cargo", "Cargo.toml", nil, []string{"cargo", "build"}, []string{"cargo", "clippy"}, []string{"cargo", "test"}},
		// Java — Maven
		{"maven", "pom.xml", nil, []string{"mvn", "compile", "-q"}, []string{"mvn", "checkstyle:check", "-q"}, []string{"mvn", "test", "-q"}},
		// Java — Gradle
		{"gradle", "build.gradle", nil, []string{"gradle", "compileJava", "-q"}, []string{"gradle", "checkstyleMain", "-q"}, []string{"gradle", "test", "-q"}},
		{"gradle-kts", "build.gradle.kts", nil, []string{"gradle", "compileJava", "-q"}, []string{"gradle", "checkstyleMain", "-q"}, []string{"gradle", "test", "-q"}},
		// Python — pip with requirements.txt
		{"pip", "requirements.txt", []string{"pip", "install", "-r", "requirements.txt"}, nil, []string{"python", "-m", "flake8", "."}, []string{"python", "-m", "pytest"}},
		// Python — Poetry
		{"poetry", "pyproject.toml", []string{"poetry", "install", "--no-interaction"}, nil, []string{"poetry", "run", "flake8", "."}, []string{"poetry", "run", "pytest"}},
		// Python — Pipenv
		{"pipenv", "Pipfile", []string{"pipenv", "install", "--dev"}, nil, []string{"pipenv", "run", "flake8", "."}, []string{"pipenv", "run", "pytest"}},
	}

	var selected *buildConfig
	for i, cfg := range configs {
		if _, err := os.Stat(filepath.Join(cloneDir, cfg.detectFile)); err == nil {
			selected = &configs[i]
			break
		}
	}

	if selected == nil {
		log.Info().Msg("no recognized build system found, skipping build verification")
		return "", nil // no build system detected — treat as success
	}

	// Check if the build tool is actually installed before attempting to run it
	toolBin := selected.installCmd
	if toolBin == nil {
		toolBin = selected.buildCmd
	}
	if toolBin != nil {
		if _, lookErr := exec.LookPath(toolBin[0]); lookErr != nil {
			log.Info().Str("build_system", selected.name).Str("tool", toolBin[0]).Msg("build tool not installed, skipping build verification")
			return "", nil
		}
	}

	log.Info().Str("build_system", selected.name).Msg("running build verification")

	const buildTimeout = 120 * time.Second
	var allOutput strings.Builder

	// Run install command if present (Node.js projects need deps installed)
	if selected.installCmd != nil {
		installCtx, installCancel := context.WithTimeout(ctx, buildTimeout)
		defer installCancel()

		cmd := exec.CommandContext(installCtx, selected.installCmd[0], selected.installCmd[1:]...)
		cmd.Dir = cloneDir
		cmd.Env = append(os.Environ(), "CI=true")
		output, err := cmd.CombinedOutput()
		allOutput.WriteString(string(output))
		if err != nil {
			allOutput.WriteString("\n\nInstall failed: " + err.Error())
			return allOutput.String(), fmt.Errorf("%s install failed: %w", selected.name, err)
		}
	}

	// Run build command (some stacks like Python don't have a compile step)
	if selected.buildCmd != nil {
		buildCtx, buildCancel := context.WithTimeout(ctx, buildTimeout)
		defer buildCancel()

		cmd := exec.CommandContext(buildCtx, selected.buildCmd[0], selected.buildCmd[1:]...)
		cmd.Dir = cloneDir
		cmd.Env = append(os.Environ(), "CI=true")
		output, err := cmd.CombinedOutput()
		allOutput.WriteString("\n")
		allOutput.WriteString(string(output))
		if err != nil {
			return allOutput.String(), fmt.Errorf("%s build failed: %w", selected.name, err)
		}
	}

	// Run lint command if available (skip gracefully if script doesn't exist)
	if selected.lintCmd != nil {
		lintCtx, lintCancel := context.WithTimeout(ctx, buildTimeout)
		defer lintCancel()

		lintExec := exec.CommandContext(lintCtx, selected.lintCmd[0], selected.lintCmd[1:]...)
		lintExec.Dir = cloneDir
		lintExec.Env = append(os.Environ(), "CI=true")
		lintOutput, lintErr := lintExec.CombinedOutput()
		allOutput.WriteString("\n")
		allOutput.WriteString(string(lintOutput))
		if lintErr != nil && !isMissingScript(string(lintOutput)) {
			return allOutput.String(), fmt.Errorf("%s lint failed: %w", selected.name, lintErr)
		}
	}

	// Run test command if available (skip gracefully if script doesn't exist)
	if selected.testCmd != nil {
		testCtx, testCancel := context.WithTimeout(ctx, buildTimeout)
		defer testCancel()

		testExec := exec.CommandContext(testCtx, selected.testCmd[0], selected.testCmd[1:]...)
		testExec.Dir = cloneDir
		testExec.Env = append(os.Environ(), "CI=true")
		testOutput, testErr := testExec.CombinedOutput()
		allOutput.WriteString("\n")
		allOutput.WriteString(string(testOutput))
		if testErr != nil && !isMissingScript(string(testOutput)) {
			return allOutput.String(), fmt.Errorf("%s tests failed: %w", selected.name, testErr)
		}
	}

	return allOutput.String(), nil
}

// isMissingScript checks if command output indicates the script/command doesn't exist.
func isMissingScript(output string) bool {
	indicators := []string{
		// Node.js (pnpm/yarn/npm)
		"Missing script", "missing script", "No such command",
		// General
		"command not found", "not recognized as",
		// Go
		"no test files", "no Go files",
		// Python
		"No module named flake8", "No module named pytest",
		// Maven/Gradle
		"Could not find goal", "Task 'checkstyleMain' not found",
		"plugin not found", "Unknown lifecycle phase",
	}
	for _, s := range indicators {
		if strings.Contains(output, s) {
			return true
		}
	}
	return false
}

// truncateString truncates a string to maxLen, adding an ellipsis if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:] + "\n... (truncated)"
}

// updateLockfile runs the package manager's lockfile update command and commits
// the updated lockfile to the branch via GitHub API.
// NOTE: code changes must already be applied to cloneDir before calling this.
func (h *AutoMigrateHandler) updateLockfile(
	ctx context.Context, token, owner, repo, branch, baseBranch, cloneDir string,
) error {
	log := logger.With("auto-migrate")

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

// validateCodeChanges filters out destructive changes where the AI replaced
// real implementations with stubs or significantly reduced file content.
func validateCodeChanges(cloneDir string, changes []agent.CodeChange) []agent.CodeChange {
	log := logger.With("auto-migrate")
	var valid []agent.CodeChange

	for _, change := range changes {
		if change.Action == "create" || change.Action == "delete" {
			valid = append(valid, change)
			continue
		}

		// For modifications, compare with original file
		origPath := filepath.Join(cloneDir, filepath.FromSlash(change.File))
		origData, err := os.ReadFile(origPath)
		if err != nil {
			// File doesn't exist — treat as create
			valid = append(valid, change)
			continue
		}

		origSize := len(origData)
		newSize := len(change.Content)

		// Reject if new content is less than 50% of original size (AI likely destroyed the file)
		if origSize > 200 && newSize < origSize/2 {
			log.Warn().
				Str("file", change.File).
				Int("original_size", origSize).
				Int("new_size", newSize).
				Msg("REJECTED: change would reduce file to less than 50% of original size — AI likely destroyed code")
			continue
		}

		// Reject if content contains placeholder stubs
		if containsPlaceholderStubs(change.Content) {
			log.Warn().
				Str("file", change.File).
				Msg("REJECTED: change contains placeholder stubs (// Implementation would...)")
			continue
		}

		valid = append(valid, change)
	}

	if len(valid) < len(changes) {
		log.Warn().
			Int("original", len(changes)).
			Int("accepted", len(valid)).
			Int("rejected", len(changes)-len(valid)).
			Msg("some code changes were rejected by validation")
	}

	return valid
}

// containsPlaceholderStubs checks if content has stub implementations.
func containsPlaceholderStubs(content string) bool {
	stubs := []string{
		"// Implementation would",
		"// Placeholder",
		"// TODO: implement",
		"// implementation would",
		"// placeholder",
	}
	count := 0
	for _, stub := range stubs {
		count += strings.Count(content, stub)
	}
	return count >= 2 // Two or more stubs = likely a destroyed file
}
