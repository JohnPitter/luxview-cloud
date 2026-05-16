package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

const (
	actionTriggerManual = "manual"
	actionTriggerPush   = "push"
	actionWorkspaceRoot = "luxview-actions"
	dockerRunTimeout    = 20 * time.Minute
	defaultArtifactName = "artifact"
	defaultArtifactPath = "."
)

var ErrActionWorkflowNotFound = errors.New("action workflow not found")

var defaultWorkflowPaths = []string{
	".github/workflows/ci.yml",
	".github/workflows/ci.yaml",
	".github/workflows/pipeline.yml",
	".github/workflows/pipeline.yaml",
	".github/workflows/build.yml",
	".github/workflows/build.yaml",
	".github/workflows/deploy.yml",
	".github/workflows/deploy.yaml",
}

type ActionService struct {
	actionRepo      *repository.ActionRepo
	appRepo         *repository.AppRepo
	repoCloner      *RepoCloner
	buildQueue      chan<- DeployRequest
	artifactBaseDir string
}

func NewActionService(actionRepo *repository.ActionRepo, appRepo *repository.AppRepo, userRepo *repository.UserRepo, encryptionKey []byte, buildQueue chan<- DeployRequest, artifactBaseDir string) *ActionService {
	if artifactBaseDir == "" {
		artifactBaseDir = filepath.Join(os.TempDir(), "luxview-action-artifacts")
	}
	return &ActionService{
		actionRepo:      actionRepo,
		appRepo:         appRepo,
		repoCloner:      NewRepoCloner(userRepo, encryptionKey, "actions"),
		buildQueue:      buildQueue,
		artifactBaseDir: artifactBaseDir,
	}
}

type TriggerActionRequest struct {
	WorkflowPath string `json:"workflow_path"`
	CommitSHA    string `json:"commit_sha"`
	Trigger      string `json:"trigger"`
}

func (s *ActionService) TriggerRun(ctx context.Context, appID uuid.UUID, req TriggerActionRequest) (*model.ActionRun, error) {
	app, err := s.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		return nil, fmt.Errorf("app not found")
	}

	trigger := req.Trigger
	if trigger == "" {
		trigger = actionTriggerManual
	}

	parseDir := filepath.Join(os.TempDir(), actionWorkspaceRoot, "parse-"+uuid.NewString())
	defer os.RemoveAll(parseDir)
	if err := s.repoCloner.Clone(ctx, app, parseDir); err != nil {
		return nil, err
	}

	workflowPath, workflowContent, err := readWorkflowFile(parseDir, req.WorkflowPath)
	if err != nil {
		return nil, err
	}
	workflow, err := ParseGitHubWorkflow(workflowContent)
	if err != nil {
		return nil, err
	}

	run := &model.ActionRun{
		AppID:        app.ID,
		Workflow:     workflow.Name,
		WorkflowPath: workflowPath,
		Trigger:      trigger,
		Branch:       app.RepoBranch,
		CommitSHA:    req.CommitSHA,
		Status:       model.ActionQueued,
	}
	jobs := make([]model.ActionJob, 0, len(workflow.Jobs))
	stepsByJob := make(map[string][]model.ActionStep, len(workflow.Jobs))
	for _, parsedJob := range workflow.Jobs {
		jobs = append(jobs, model.ActionJob{
			Name:   parsedJob.Name,
			Image:  parsedJob.Image,
			Status: model.ActionQueued,
		})
		stepsByJob[parsedJob.Name] = parsedJob.Steps
	}

	if err := s.actionRepo.CreateRun(ctx, run, jobs, stepsByJob); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *ActionService) ExecuteRun(ctx context.Context, run *model.ActionRun) error {
	log := logger.With("actions")
	app, err := s.appRepo.FindByID(ctx, run.AppID)
	if err != nil || app == nil {
		_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
		return fmt.Errorf("app not found")
	}

	workspace := filepath.Join(os.TempDir(), actionWorkspaceRoot, run.ID.String())
	defer os.RemoveAll(workspace)
	if err := s.repoCloner.Clone(ctx, app, workspace); err != nil {
		_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
		return err
	}

	secrets, err := s.actionRepo.GetSecretValues(ctx, app.ID)
	if err != nil {
		_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
		return err
	}

	jobs, err := s.actionRepo.ListJobsForRun(ctx, run.ID)
	if err != nil {
		_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
		return err
	}

	for _, job := range jobs {
		if err := s.executeJob(ctx, run.ID, workspace, job, secrets); err != nil {
			_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
			return err
		}
	}

	if err := s.queueDeployAfterPush(ctx, app, run); err != nil {
		_ = s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionFailed)
		return err
	}

	log.Info().Str("run_id", run.ID.String()).Msg("action run completed")
	return s.actionRepo.UpdateRunStatus(ctx, run.ID, model.ActionSuccess)
}

func (s *ActionService) queueDeployAfterPush(ctx context.Context, app *model.App, run *model.ActionRun) error {
	if run.Trigger != actionTriggerPush || !app.AutoDeploy || s.buildQueue == nil {
		return nil
	}

	req := DeployRequest{
		AppID:     app.ID,
		UserID:    app.UserID,
		CommitSHA: run.CommitSHA,
		CommitMsg: "action workflow passed: " + run.Workflow,
		Source:    "action",
	}

	select {
	case s.buildQueue <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("build queue full")
	}
}

func (s *ActionService) executeJob(ctx context.Context, runID uuid.UUID, workspace string, job model.ActionJob, secrets map[string]string) error {
	if err := s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionRunning); err != nil {
		return err
	}

	steps, err := s.actionRepo.ListStepsForJob(ctx, job.ID)
	if err != nil {
		_ = s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionFailed)
		return err
	}

	for _, step := range steps {
		if err := s.actionRepo.StartStep(ctx, step.ID); err != nil {
			_ = s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionFailed)
			return err
		}
		status, exitCode, output := s.executeStep(ctx, runID, workspace, job.Image, step, secrets)
		output = maskSecretValues(output, secrets)
		if err := s.actionRepo.UpdateStepResult(ctx, step.ID, status, exitCode, output); err != nil {
			_ = s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionFailed)
			return err
		}
		if status != model.ActionSuccess {
			_ = s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionFailed)
			return fmt.Errorf("action step failed: %s", step.Name)
		}
	}

	return s.actionRepo.UpdateJobStatus(ctx, job.ID, model.ActionSuccess)
}

func (s *ActionService) executeStep(ctx context.Context, runID uuid.UUID, workspace, image string, step model.ActionStep, secrets map[string]string) (model.ActionStatus, int, string) {
	if step.Kind == actionKindUses {
		return s.executeUsesStep(ctx, runID, workspace, step)
	}
	if strings.TrimSpace(step.Command) == "" {
		return model.ActionSuccess, 0, "empty command skipped\n"
	}

	stepCtx, cancel := context.WithTimeout(ctx, dockerRunTimeout)
	defer cancel()

	args := []string{
		"run", "--rm",
		"--memory=1g",
		"--cpus=1",
		"-v", fmt.Sprintf("%s:/workspace", workspace),
		"-w", "/workspace",
	}
	for _, key := range sortedSecretKeys(secrets) {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, secrets[key]))
	}
	args = append(args, image, "sh", "-lc", step.Command)
	cmd := exec.CommandContext(stepCtx, "docker", args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err == nil {
		return model.ActionSuccess, 0, output.String()
	}

	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	if stepCtx.Err() == context.DeadlineExceeded {
		return model.ActionFailed, exitCode, output.String() + "\n[timeout] step exceeded timeout\n"
	}
	return model.ActionFailed, exitCode, output.String() + "\n" + err.Error() + "\n"
}

func (s *ActionService) executeUsesStep(ctx context.Context, runID uuid.UUID, workspace string, step model.ActionStep) (model.ActionStatus, int, string) {
	switch {
	case step.Uses == "":
		return model.ActionSuccess, 0, "metadata step skipped\n"
	case strings.HasPrefix(step.Uses, "actions/checkout@"):
		return model.ActionSuccess, 0, "checkout handled by LuxView before job execution\n"
	case strings.HasPrefix(step.Uses, "actions/setup-node@"):
		return model.ActionSuccess, 0, "setup-node mapped to job Docker image\n"
	case strings.HasPrefix(step.Uses, "actions/setup-go@"):
		return model.ActionSuccess, 0, "setup-go mapped to job Docker image\n"
	case strings.HasPrefix(step.Uses, "actions/setup-java@"):
		return model.ActionSuccess, 0, "setup-java mapped to job Docker image\n"
	case strings.HasPrefix(step.Uses, "actions/upload-artifact@"):
		return s.uploadArtifact(ctx, runID, workspace, step)
	case strings.HasPrefix(step.Uses, "actions/download-artifact@"):
		return s.downloadArtifact(ctx, runID, workspace, step)
	default:
		return model.ActionFailed, 1, "unsupported action: " + step.Uses + "\n"
	}
}

func (s *ActionService) uploadArtifact(ctx context.Context, runID uuid.UUID, workspace string, step model.ActionStep) (model.ActionStatus, int, string) {
	name := step.Inputs["name"]
	if name == "" {
		name = defaultArtifactName
	}
	artifactPath := step.Inputs["path"]
	if artifactPath == "" {
		artifactPath = defaultArtifactPath
	}

	sources, err := expandArtifactSources(workspace, artifactPath)
	if err != nil {
		return model.ActionFailed, 1, err.Error() + "\n"
	}
	if len(sources) == 0 {
		return model.ActionFailed, 1, fmt.Sprintf("artifact path not found: %s\n", artifactPath)
	}

	destDir := filepath.Join(s.artifactBaseDir, runID.String())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return model.ActionFailed, 1, fmt.Sprintf("create artifact directory: %v\n", err)
	}
	destPath := filepath.Join(destDir, sanitizeArtifactName(name))
	_ = os.RemoveAll(destPath)
	if err := copyArtifactSources(sources, destPath); err != nil {
		return model.ActionFailed, 1, fmt.Sprintf("copy artifact: %v\n", err)
	}
	sizeBytes, err := pathSize(destPath)
	if err != nil {
		return model.ActionFailed, 1, fmt.Sprintf("measure artifact: %v\n", err)
	}

	artifact := &model.ActionArtifact{
		RunID:     runID,
		Name:      name,
		Path:      destPath,
		SizeBytes: sizeBytes,
	}
	if err := s.actionRepo.UpsertArtifact(ctx, artifact); err != nil {
		return model.ActionFailed, 1, err.Error() + "\n"
	}
	return model.ActionSuccess, 0, fmt.Sprintf("uploaded artifact %q (%d bytes, %d path(s))\n", name, sizeBytes, len(sources))
}

func (s *ActionService) downloadArtifact(ctx context.Context, runID uuid.UUID, workspace string, step model.ActionStep) (model.ActionStatus, int, string) {
	name := step.Inputs["name"]
	if name == "" {
		name = defaultArtifactName
	}
	destInput := step.Inputs["path"]
	if destInput == "" {
		destInput = defaultArtifactPath
	}
	destPath, err := safeWorkspacePath(workspace, destInput)
	if err != nil {
		return model.ActionFailed, 1, err.Error() + "\n"
	}

	artifact, err := s.actionRepo.FindArtifact(ctx, runID, name)
	if err != nil {
		return model.ActionFailed, 1, err.Error() + "\n"
	}
	if artifact == nil {
		return model.ActionFailed, 1, fmt.Sprintf("artifact not found: %s\n", name)
	}
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return model.ActionFailed, 1, fmt.Sprintf("create destination: %v\n", err)
	}
	targetPath := filepath.Join(destPath, sanitizeArtifactName(name))
	_ = os.RemoveAll(targetPath)
	if err := copyPath(artifact.Path, targetPath); err != nil {
		return model.ActionFailed, 1, fmt.Sprintf("download artifact: %v\n", err)
	}
	return model.ActionSuccess, 0, fmt.Sprintf("downloaded artifact %q to %s\n", name, destInput)
}

func readWorkflowFile(repoDir, requestedPath string) (string, string, error) {
	if requestedPath != "" {
		content, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(requestedPath)))
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("%w: %s", ErrActionWorkflowNotFound, requestedPath)
		}
		if err != nil {
			return "", "", fmt.Errorf("read workflow %s: %w", requestedPath, err)
		}
		return requestedPath, string(content), nil
	}

	for _, candidate := range defaultWorkflowPaths {
		content, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(candidate)))
		if err == nil {
			return candidate, string(content), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("read workflow %s: %w", candidate, err)
		}
	}

	return "", "", ErrActionWorkflowNotFound
}

func sortedSecretKeys(secrets map[string]string) []string {
	keys := make([]string, 0, len(secrets))
	for key := range secrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func maskSecretValues(log string, secrets map[string]string) string {
	masked := log
	for _, value := range secrets {
		if len(value) < 4 {
			continue
		}
		masked = strings.ReplaceAll(masked, value, "***")
	}
	return masked
}

func IsValidActionSecretKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		validLetter := r >= 'A' && r <= 'Z'
		validDigit := r >= '0' && r <= '9'
		if i == 0 && validDigit {
			return false
		}
		if !validLetter && !validDigit && r != '_' {
			return false
		}
	}
	return true
}

func safeWorkspacePath(workspace, relativePath string) (string, error) {
	cleanRelative := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanRelative) || strings.HasPrefix(cleanRelative, "..") {
		return "", fmt.Errorf("path escapes workspace: %s", relativePath)
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(workspaceAbs, cleanRelative))
	if err != nil {
		return "", err
	}
	if targetAbs != workspaceAbs && !strings.HasPrefix(targetAbs, workspaceAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace: %s", relativePath)
	}
	return targetAbs, nil
}

type artifactSource struct {
	AbsPath string
	RelPath string
}

func expandArtifactSources(workspace, input string) ([]artifactSource, error) {
	var sources []artifactSource
	seen := make(map[string]bool)
	for _, pattern := range artifactPathPatterns(input) {
		matches, err := expandArtifactPattern(workspace, pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if seen[match.AbsPath] {
				continue
			}
			seen[match.AbsPath] = true
			sources = append(sources, match)
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].RelPath < sources[j].RelPath
	})
	return sources, nil
}

func artifactPathPatterns(input string) []string {
	lines := strings.Split(input, "\n")
	var patterns []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func expandArtifactPattern(workspace, pattern string) ([]artifactSource, error) {
	if !hasGlob(pattern) {
		absPath, err := safeWorkspacePath(workspace, pattern)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []artifactSource{{AbsPath: absPath, RelPath: filepath.ToSlash(filepath.Clean(pattern))}}, nil
	}

	if _, err := safeWorkspacePath(workspace, globSafetyBase(pattern)); err != nil {
		return nil, err
	}

	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	var matches []artifactSource
	err = filepath.Walk(workspaceAbs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == workspaceAbs {
			return nil
		}
		rel, err := filepath.Rel(workspaceAbs, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if matchArtifactPattern(filepath.ToSlash(pattern), relSlash) {
			matches = append(matches, artifactSource{AbsPath: path, RelPath: relSlash})
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return matches, err
}

func hasGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func globSafetyBase(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx == -1 {
		return pattern
	}
	base := pattern[:idx]
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "\\")
	if base == "" {
		return "."
	}
	return base
}

func matchArtifactPattern(pattern, rel string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := pathpkg.Match(pattern, rel)
		return ok
	}

	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")
	if prefix != "" && rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
		return false
	}
	if suffix == "" {
		return true
	}
	ok, _ := pathpkg.Match(suffix, pathpkg.Base(rel))
	return ok
}

func copyArtifactSources(sources []artifactSource, destPath string) error {
	if len(sources) == 1 {
		return copyPath(sources[0].AbsPath, destPath)
	}
	for _, source := range sources {
		targetPath := filepath.Join(destPath, filepath.FromSlash(source.RelPath))
		if err := copyPath(source.AbsPath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeArtifactName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" || name == "." || name == ".." {
		return defaultArtifactName
	}
	return name
}

func copyPath(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest)
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func pathSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
