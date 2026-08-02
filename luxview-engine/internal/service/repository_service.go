package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

const (
	defaultRepositoryBranch   = "main"
	defaultRepositoryBaseDir  = "/data/luxview/repositories"
	gitDirectoryMode          = 0755
	defaultRepositoryMaxBytes = 10 * 1024 * 1024 * 1024
)

var repositorySlugRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,118}[a-z0-9])?$`)
var gitBranchSyntaxRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
var gitTokenURLRegex = regexp.MustCompile(`https://[^\s/@]+@`)

type RepositoryStore interface {
	Create(ctx context.Context, repo *model.Repository) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Repository, error)
	FindByUserAndSlug(ctx context.Context, userID uuid.UUID, slug string) (*model.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CreateRemote(ctx context.Context, remote *model.RepositoryRemote) error
	ListRemotes(ctx context.Context, repositoryID uuid.UUID) ([]model.RepositoryRemote, error)
	UpdateRemoteSyncStatus(ctx context.Context, remoteID uuid.UUID, status model.RepositorySyncStatus, errMsg string) error
	MarkBackupsPending(ctx context.Context, repositoryID uuid.UUID) error
}

// BackupTokenProvider retrieves a GitHub token for the repository owner to use during backup push.
type BackupTokenProvider interface {
	TokenForUser(ctx context.Context, user *model.User) (string, error)
}

// UserLookup retrieves a user by ID.
type UserLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type RepositoryService struct {
	store         RepositoryStore
	baseDir       string
	maxRepoBytes  int64
	backupMu      sync.Mutex
	tokenProvider BackupTokenProvider
	userLookup    UserLookup
}

type CreateRepositoryRequest struct {
	UserID        uuid.UUID
	Name          string
	Slug          string
	Description   string
	DefaultBranch string
	Visibility    model.RepositoryVisibility
}

func NewRepositoryService(store RepositoryStore, baseDir string) *RepositoryService {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = defaultRepositoryBaseDir
	}
	return &RepositoryService{store: store, baseDir: baseDir, maxRepoBytes: defaultRepositoryMaxBytes}
}

func (s *RepositoryService) WithMaxRepositoryBytes(maxBytes int64) {
	if maxBytes > 0 {
		s.maxRepoBytes = maxBytes
	}
}

// WithBackupSupport attaches a token provider and user lookup so SyncBackup can authenticate.
func (s *RepositoryService) WithBackupSupport(tokenProvider BackupTokenProvider, userLookup UserLookup) {
	s.tokenProvider = tokenProvider
	s.userLookup = userLookup
}

func (s *RepositoryService) Create(ctx context.Context, req CreateRepositoryRequest) (*model.Repository, error) {
	if s.store == nil {
		return nil, fmt.Errorf("repository store is required")
	}
	if req.UserID == uuid.Nil {
		return nil, fmt.Errorf("user_id is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	slug := normalizeRepositorySlug(req.Slug)
	if slug == "" {
		slug = normalizeRepositorySlug(name)
	}
	if !repositorySlugRegex.MatchString(slug) {
		return nil, fmt.Errorf("invalid repository slug")
	}
	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = defaultRepositoryBranch
	}
	if err := validateGitBranchSyntax(defaultBranch); err != nil {
		return nil, err
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = model.RepositoryVisibilityPrivate
	}
	if visibility != model.RepositoryVisibilityPrivate && visibility != model.RepositoryVisibilityPublic {
		return nil, fmt.Errorf("invalid repository visibility")
	}

	repoID := uuid.New()
	storagePath := s.repositoryPath(req.UserID, repoID)
	repo := &model.Repository{
		ID:            repoID,
		UserID:        req.UserID,
		Name:          name,
		Slug:          slug,
		Description:   strings.TrimSpace(req.Description),
		DefaultBranch: defaultBranch,
		StoragePath:   storagePath,
		Visibility:    visibility,
	}

	if err := os.MkdirAll(filepath.Dir(storagePath), gitDirectoryMode); err != nil {
		return nil, fmt.Errorf("create repository parent directory: %w", err)
	}
	if err := runGit(ctx, "", "init", "--bare", "--initial-branch", defaultBranch, storagePath); err != nil {
		return nil, fmt.Errorf("initialize repository: %w", err)
	}
	if err := s.ensureReceiveHook(storagePath, nil); err != nil {
		_ = os.RemoveAll(storagePath)
		return nil, fmt.Errorf("configure repository hooks: %w", err)
	}
	if s.store != nil {
		if err := s.store.Create(ctx, repo); err != nil {
			_ = os.RemoveAll(storagePath)
			return nil, err
		}
	}
	return repo, nil
}

type ImportRepositoryRequest struct {
	UserID        uuid.UUID
	Name          string
	Slug          string
	DefaultBranch string
	Visibility    model.RepositoryVisibility
	// RemoteURL is the authenticated clone URL (token already embedded or public)
	RemoteURL string
}

// ImportFromGitHub clones an existing GitHub repository into LuxView-hosted storage.
// The caller must embed the token into RemoteURL before calling (e.g. https://TOKEN@github.com/...).
func (s *RepositoryService) ImportFromRemote(ctx context.Context, req ImportRepositoryRequest) (*model.Repository, error) {
	if s.store == nil {
		return nil, fmt.Errorf("repository store is required")
	}
	if req.UserID == uuid.Nil {
		return nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.RemoteURL) == "" {
		return nil, fmt.Errorf("remote_url is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	slug := normalizeRepositorySlug(req.Slug)
	if slug == "" {
		slug = normalizeRepositorySlug(name)
	}
	if !repositorySlugRegex.MatchString(slug) {
		return nil, fmt.Errorf("invalid repository slug")
	}
	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = defaultRepositoryBranch
	}
	if err := validateGitBranchSyntax(defaultBranch); err != nil {
		return nil, err
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = model.RepositoryVisibilityPrivate
	}

	repoID := uuid.New()
	storagePath := s.repositoryPath(req.UserID, repoID)

	if err := os.MkdirAll(filepath.Dir(storagePath), gitDirectoryMode); err != nil {
		return nil, fmt.Errorf("create repository parent directory: %w", err)
	}

	// Clone as a bare mirror — fetches all refs (branches, tags)
	if err := runGit(ctx, "", "clone", "--mirror", req.RemoteURL, storagePath); err != nil {
		_ = os.RemoveAll(storagePath)
		return nil, fmt.Errorf("clone remote repository: %w", err)
	}
	if err := s.ensureReceiveHook(storagePath, nil); err != nil {
		_ = os.RemoveAll(storagePath)
		return nil, fmt.Errorf("configure repository hooks: %w", err)
	}

	repo := &model.Repository{
		ID:            repoID,
		UserID:        req.UserID,
		Name:          name,
		Slug:          slug,
		DefaultBranch: defaultBranch,
		StoragePath:   storagePath,
		Visibility:    visibility,
	}

	if s.store != nil {
		if err := s.store.Create(ctx, repo); err != nil {
			_ = os.RemoveAll(storagePath)
			return nil, err
		}
	}
	return repo, nil
}

// ImportFromGitHub resolves a GitHub token for the user and imports the repo.
// owner/repoName must be the GitHub repository (e.g. "octocat/Hello-World").
func (s *RepositoryService) ImportFromGitHub(ctx context.Context, userID uuid.UUID, owner, repoName, defaultBranch string, visibility model.RepositoryVisibility) (*model.Repository, error) {
	if s.tokenProvider == nil || s.userLookup == nil {
		return nil, fmt.Errorf("GitHub token provider not configured — connect GitHub App first")
	}
	user, err := s.userLookup.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	token, err := s.tokenProvider.TokenForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("could not get GitHub token: %w", err)
	}
	remoteURL := fmt.Sprintf("https://%s@github.com/%s/%s.git", token, owner, repoName)
	name := repoName
	return s.ImportFromRemote(ctx, ImportRepositoryRequest{
		UserID:        userID,
		Name:          name,
		Slug:          normalizeRepositorySlug(repoName),
		DefaultBranch: defaultBranch,
		Visibility:    visibility,
		RemoteURL:     remoteURL,
	})
}

func (s *RepositoryService) Delete(ctx context.Context, repositoryID uuid.UUID, userID uuid.UUID) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if repo.UserID != userID {
		return fmt.Errorf("forbidden")
	}
	if err := os.RemoveAll(repo.StoragePath); err != nil {
		return fmt.Errorf("remove repository storage: %w", err)
	}
	if err := s.store.Delete(ctx, repositoryID); err != nil {
		return fmt.Errorf("delete repository metadata: %w", err)
	}
	return nil
}

func (s *RepositoryService) ListBranches(ctx context.Context, repositoryID uuid.UUID) ([]string, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	output, err := gitOutput(ctx, repo.StoragePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	branches := splitGitLines(output)
	sort.Strings(branches)
	return branches, nil
}

func (s *RepositoryService) ResolveRef(ctx context.Context, repositoryID uuid.UUID, ref string) (string, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return "", err
	}
	normalizedRef := strings.TrimSpace(ref)
	if normalizedRef == "" {
		normalizedRef = repo.DefaultBranch
	}
	if err := validateGitRefArgument(normalizedRef); err != nil {
		return "", err
	}
	commit, err := gitOutput(ctx, repo.StoragePath, "rev-parse", "--verify", "--end-of-options", normalizedRef+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", normalizedRef, err)
	}
	return strings.TrimSpace(commit), nil
}

func (s *RepositoryService) Checkout(ctx context.Context, repositoryID uuid.UUID, ref, destDir string) (*model.CheckoutResult, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(destDir) == "" {
		return nil, fmt.Errorf("destination directory is required")
	}
	if err := os.RemoveAll(destDir); err != nil {
		return nil, fmt.Errorf("clean checkout directory: %w", err)
	}
	if err := runGit(ctx, "", "clone", repo.StoragePath, destDir); err != nil {
		return nil, fmt.Errorf("clone repository: %w", err)
	}

	normalizedRef := strings.TrimSpace(ref)
	if normalizedRef == "" {
		normalizedRef = repo.DefaultBranch
	}
	if err := validateGitRefArgument(normalizedRef); err != nil {
		return nil, err
	}
	if err := runGit(ctx, destDir, "checkout", normalizedRef); err != nil {
		return nil, fmt.Errorf("checkout ref %q: %w", normalizedRef, err)
	}
	commit, err := gitOutput(ctx, destDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve checkout head: %w", err)
	}

	return &model.CheckoutResult{
		RepositoryID: repo.ID,
		Ref:          normalizedRef,
		CommitSHA:    strings.TrimSpace(commit),
		WorkDir:      destDir,
	}, nil
}

// ConfigureBackupRemote adds a GitHub remote entry and registers it in the bare repository.
// remoteURL must be the canonical HTTPS URL (e.g. https://github.com/owner/repo.git).
// The token is stored externally; the remote URL in git config is set with the token embedded only during sync.
func (s *RepositoryService) ConfigureBackupRemote(ctx context.Context, repositoryID uuid.UUID, provider, remoteURL string) (*model.RepositoryRemote, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	remoteURL = strings.TrimSpace(remoteURL)
	if err := validateBackupRemoteURL(remoteURL); err != nil {
		return nil, err
	}
	remote := &model.RepositoryRemote{
		RepositoryID: repo.ID,
		Provider:     provider,
		RemoteURL:    remoteURL,
		Mode:         model.RepositoryRemoteModeBackup,
	}
	if err := s.store.CreateRemote(ctx, remote); err != nil {
		return nil, fmt.Errorf("create remote record: %w", err)
	}
	return remote, nil
}

// ListRemotes returns the configured backup remotes for a repository.
func (s *RepositoryService) ListRemotes(ctx context.Context, repositoryID uuid.UUID) ([]model.RepositoryRemote, error) {
	return s.store.ListRemotes(ctx, repositoryID)
}

func (s *RepositoryService) MarkBackupsPending(ctx context.Context, repositoryID uuid.UUID) error {
	return s.store.MarkBackupsPending(ctx, repositoryID)
}

// SyncBackup pushes all refs to the backup remote.
// userID is the repository owner — used to retrieve the GitHub token.
// Failure is non-fatal by design: it updates the sync status but does not propagate the error.
func (s *RepositoryService) SyncBackup(ctx context.Context, repositoryID uuid.UUID, remoteID uuid.UUID, userID uuid.UUID) error {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}

	remotes, err := s.store.ListRemotes(ctx, repositoryID)
	if err != nil {
		return fmt.Errorf("list remotes: %w", err)
	}
	var target *model.RepositoryRemote
	for i := range remotes {
		if remotes[i].ID == remoteID {
			target = &remotes[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("remote not found")
	}
	if err := s.store.UpdateRemoteSyncStatus(ctx, remoteID, model.RepositorySyncStatusPending, ""); err != nil {
		return fmt.Errorf("mark backup pending: %w", err)
	}

	pushURL, err := s.buildAuthURL(ctx, target.RemoteURL, userID)
	if err != nil {
		syncErr := fmt.Sprintf("build auth url: %s", err.Error())
		_ = s.store.UpdateRemoteSyncStatus(ctx, remoteID, model.RepositorySyncStatusFailed, syncErr)
		return fmt.Errorf("backup auth: %w", err)
	}

	if err := runGit(ctx, repo.StoragePath, "push", "--mirror", pushURL); err != nil {
		syncErr := err.Error()
		_ = s.store.UpdateRemoteSyncStatus(ctx, remoteID, model.RepositorySyncStatusFailed, syncErr)
		return fmt.Errorf("backup push: %w", err)
	}

	return s.store.UpdateRemoteSyncStatus(ctx, remoteID, model.RepositorySyncStatusSuccess, "")
}

// SyncAllBackups pushes to all configured backup remotes for a repository (fire-and-forget use).
func (s *RepositoryService) SyncAllBackups(ctx context.Context, repositoryID uuid.UUID, userID uuid.UUID) {
	remotes, err := s.store.ListRemotes(ctx, repositoryID)
	if err != nil {
		return
	}
	for _, remote := range remotes {
		_ = s.SyncBackup(ctx, repositoryID, remote.ID, userID)
	}
}

func (s *RepositoryService) buildAuthURL(ctx context.Context, remoteURL string, userID uuid.UUID) (string, error) {
	if s.tokenProvider == nil || s.userLookup == nil {
		return "", fmt.Errorf("backup token provider not configured")
	}
	user, err := s.userLookup.FindByID(ctx, userID)
	if err != nil || user == nil {
		return "", fmt.Errorf("user not found")
	}
	token, err := s.tokenProvider.TokenForUser(ctx, user)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(remoteURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("remote URL must be a credential-free HTTPS URL")
	}
	parsed.User = url.User(token)
	return parsed.String(), nil
}

func (s *RepositoryService) findRepository(ctx context.Context, repositoryID uuid.UUID) (*model.Repository, error) {
	if repositoryID == uuid.Nil {
		return nil, fmt.Errorf("repository_id is required")
	}
	if s.store == nil {
		return nil, fmt.Errorf("repository store is required")
	}
	repo, err := s.store.FindByID(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("repository not found")
	}
	return repo, nil
}

func (s *RepositoryService) repositoryPath(userID, repoID uuid.UUID) string {
	return filepath.Join(s.baseDir, userID.String(), repoID.String()+".git")
}

func (s *RepositoryService) EnsureReceiveHook(ctx context.Context, repositoryID uuid.UUID, rules []model.BranchProtectionRule) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	return s.ensureReceiveHook(repo.StoragePath, rules)
}

func (s *RepositoryService) CheckRepositoryCapacity(ctx context.Context, repositoryID uuid.UUID) error {
	if s.maxRepoBytes <= 0 {
		return nil
	}
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	output, err := gitOutput(ctx, repo.StoragePath, "count-objects", "-v")
	if err != nil {
		return fmt.Errorf("measure repository storage: %w", err)
	}
	used := gitObjectStorageBytes(output)
	if used >= s.maxRepoBytes {
		return fmt.Errorf("repository storage quota exceeded (%d bytes)", s.maxRepoBytes)
	}
	return nil
}

func (s *RepositoryService) ensureReceiveHook(storagePath string, rules []model.BranchProtectionRule) error {
	if strings.TrimSpace(storagePath) == "" {
		return fmt.Errorf("repository storage path is required")
	}
	hooksPath := filepath.Join(storagePath, "hooks")
	if err := os.MkdirAll(hooksPath, gitDirectoryMode); err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(hooksPath, "pre-receive"), []byte(s.receiveHookScript()), 0755); err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(storagePath, "luxview-branch-protection"), []byte(formatBranchProtectionPolicy(rules)), 0644)
}

func (s *RepositoryService) receiveHookScript() string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

git_dir=$(git rev-parse --git-dir)
policy="$git_dir/luxview-branch-protection"
max_bytes=%d

repository_size() {
  size_kb=$(git count-objects -v | awk -F': ' '$1 == "size" || $1 == "size-pack" || $1 == "size-garbage" { total += $2 } END { print total + 0 }')
  echo $((size_kb * 1024))
}

while IFS=' ' read -r oldrev newrev refname; do
  case "$refname" in
    refs/heads/*)
      branch=${refname#refs/heads/}
      if [ -f "$policy" ]; then
        while IFS='|' read -r protected require_reviews required_approvals require_checks block_force; do
          [ "$protected" = "$branch" ] || continue
          if [ "${LUXVIEW_INTERNAL_MERGE:-0}" != "1" ] && { [ "$require_reviews" = "1" ] || [ "$require_checks" = "1" ]; }; then
            echo "push rejected: branch '$branch' requires a pull request" >&2
            exit 1
          fi
          if [ "${LUXVIEW_INTERNAL_MERGE:-0}" != "1" ] && [ "$block_force" = "1" ] && [ "$oldrev" != "0000000000000000000000000000000000000000" ] && [ "$newrev" != "0000000000000000000000000000000000000000" ] && ! git merge-base --is-ancestor "$oldrev" "$newrev"; then
            echo "push rejected: force-push is disabled for branch '$branch'" >&2
            exit 1
          fi
        done < "$policy"
      fi
      ;;
  esac
done

if [ "$max_bytes" -gt 0 ] && [ "$(repository_size)" -gt "$max_bytes" ]; then
  echo "push rejected: repository storage quota exceeded" >&2
  exit 1
fi
`, s.maxRepoBytes)
}

func formatBranchProtectionPolicy(rules []model.BranchProtectionRule) string {
	var lines []string
	for _, rule := range rules {
		if validateGitBranchSyntax(rule.Branch) != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s|%s|%d|%s|%s", rule.Branch, boolFlag(rule.RequireReviews), rule.RequiredApprovals, boolFlag(rule.RequireStatusChecks), boolFlag(rule.BlockForcePush)))
	}
	return strings.Join(lines, "\n") + "\n"
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if runtime.GOOS == "windows" {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				_ = os.Remove(tmpPath)
				return err
			}
			if retryErr := os.Rename(tmpPath, path); retryErr == nil {
				return nil
			}
		}
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func gitObjectStorageBytes(output string) int64 {
	var kilobytes int64
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		switch strings.TrimSpace(parts[0]) {
		case "size", "size-pack", "size-garbage":
			var value int64
			if _, err := fmt.Sscan(strings.TrimSpace(parts[1]), &value); err == nil {
				kilobytes += value
			}
		}
	}
	return kilobytes * 1024
}

func normalizeRepositorySlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '.':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func runGit(ctx context.Context, dir string, args ...string) error {
	return runGitWithEnv(ctx, dir, nil, args...)
}

func runGitWithEnv(ctx context.Context, dir string, extraEnv map[string]string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %s: %w", redactGitArgs(args), redactGitOutput(string(output)), err)
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s: %w", redactGitArgs(args), redactGitOutput(string(output)), err)
	}
	return string(output), nil
}

func redactGitArgs(args []string) string {
	redacted := make([]string, len(args))
	for i, arg := range args {
		redacted[i] = redactGitOutput(arg)
	}
	return strings.Join(redacted, " ")
}

func redactGitOutput(value string) string {
	return gitTokenURLRegex.ReplaceAllString(value, "https://***@")
}

func validateGitBranchSyntax(branch string) error {
	branch = strings.TrimSpace(branch)
	if !gitBranchSyntaxRegex.MatchString(branch) || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.Contains(branch, "//") || strings.HasSuffix(branch, ".lock") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, "/") {
		return fmt.Errorf("invalid branch name")
	}
	return nil
}

func ValidateGitBranchName(branch string) error {
	return validateGitBranchSyntax(branch)
}

func validateRepositoryPath(path string) error {
	if path == "" || strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\\\x00\r\n") {
		return fmt.Errorf("invalid repository path")
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid repository path")
		}
	}
	return nil
}

func validateGitRefArgument(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\r\n") {
		return fmt.Errorf("invalid Git ref")
	}
	if strings.Contains(ref, "^{") || strings.Contains(ref, ":") {
		return fmt.Errorf("invalid Git ref")
	}
	return nil
}

func validateBackupRemoteURL(remoteURL string) error {
	parsed, err := url.Parse(remoteURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("remote URL must be a credential-free HTTPS URL")
	}
	return nil
}

func splitGitLines(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// ListTree returns directory entries at path for the given ref. path="" means root.
func (s *RepositoryService) ListTree(ctx context.Context, repositoryID uuid.UUID, ref, path string) ([]model.TreeEntry, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ref) == "" {
		ref = repo.DefaultBranch
	}
	if err := validateGitRefArgument(ref); err != nil {
		return nil, err
	}
	if path != "" {
		if err := validateRepositoryPath(path); err != nil {
			return nil, err
		}
	}
	treeRef := ref + ":" + path
	output, err := gitOutput(ctx, repo.StoragePath, "ls-tree", "-l", treeRef)
	if err != nil {
		// empty tree or path not found
		return []model.TreeEntry{}, nil
	}
	var entries []model.TreeEntry
	for _, line := range splitGitLines(output) {
		// format: <mode> SP <type> SP <sha> SP <size|-> TAB <name>
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		meta := strings.Fields(parts[0])
		if len(meta) < 4 {
			continue
		}
		mode, typ, name := meta[0], meta[1], parts[1]
		size := int64(0)
		if meta[3] != "-" {
			fmt.Sscanf(meta[3], "%d", &size)
		}
		entryPath := name
		if path != "" {
			entryPath = path + "/" + name
		}
		entries = append(entries, model.TreeEntry{
			Type: typ,
			Name: name,
			Path: entryPath,
			Size: size,
			Mode: mode,
		})
	}
	// Sort: trees first, then blobs, both alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "tree"
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// GetBlob returns the raw content of a file at path for the given ref.
func (s *RepositoryService) GetBlob(ctx context.Context, repositoryID uuid.UUID, ref, path string) ([]byte, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ref) == "" {
		ref = repo.DefaultBranch
	}
	if err := validateGitRefArgument(ref); err != nil {
		return nil, err
	}
	if err := validateRepositoryPath(path); err != nil {
		return nil, err
	}
	output, err := gitOutput(ctx, repo.StoragePath, "show", ref+":"+path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return []byte(output), nil
}

// ListCommits returns commit history for the given ref, limit/offset pagination.
func (s *RepositoryService) ListCommits(ctx context.Context, repositoryID uuid.UUID, ref string, limit, offset int) ([]model.CommitEntry, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ref) == "" {
		ref = repo.DefaultBranch
	}
	if err := validateGitRefArgument(ref); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	skipArg := fmt.Sprintf("--skip=%d", offset)
	nArg := fmt.Sprintf("-n%d", limit)
	output, err := gitOutput(ctx, repo.StoragePath, "log", ref, "--format=%H|%s|%an|%ae|%aI", nArg, skipArg)
	if err != nil {
		return []model.CommitEntry{}, nil
	}
	var commits []model.CommitEntry
	for _, line := range splitGitLines(output) {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, model.CommitEntry{
			SHA:     parts[0],
			Message: parts[1],
			Author:  parts[2],
			Email:   parts[3],
			Date:    parts[4],
		})
	}
	return commits, nil
}

// GetCommit returns detailed info + diff for a single commit.
func (s *RepositoryService) GetCommit(ctx context.Context, repositoryID uuid.UUID, sha string) (*model.CommitEntry, []model.PRFileDiff, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, nil, err
	}
	if err := validateGitRefArgument(sha); err != nil {
		return nil, nil, err
	}
	// Header
	headerOut, err := gitOutput(ctx, repo.StoragePath, "show", "--no-patch", "--format=%H|%s|%an|%ae|%aI", sha)
	if err != nil {
		return nil, nil, fmt.Errorf("commit not found: %s", sha)
	}
	line := strings.TrimSpace(strings.SplitN(headerOut, "\n", 2)[0])
	parts := strings.SplitN(line, "|", 5)
	if len(parts) < 5 {
		return nil, nil, fmt.Errorf("unexpected commit format")
	}
	entry := &model.CommitEntry{SHA: parts[0], Message: parts[1], Author: parts[2], Email: parts[3], Date: parts[4]}

	// numstat for stats
	numstatOut, _ := gitOutput(ctx, repo.StoragePath, "show", "--numstat", "--format=", sha)
	fileStats := map[string][2]int{}
	for _, l := range splitGitLines(numstatOut) {
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		var add, del int
		fmt.Sscanf(f[0], "%d", &add)
		fmt.Sscanf(f[1], "%d", &del)
		fileStats[f[2]] = [2]int{add, del}
	}

	// unified diff
	patchOut, _ := gitOutput(ctx, repo.StoragePath, "show", "--unified=3", "--format=", sha)
	diffs := parsePatch(patchOut, fileStats)
	return entry, diffs, nil
}

// ListTags returns all tags in the repository.
func (s *RepositoryService) ListTags(ctx context.Context, repositoryID uuid.UUID) ([]model.TagEntry, error) {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	output, err := gitOutput(ctx, repo.StoragePath, "for-each-ref", "--format=%(refname:short)|%(objecttype)|%(objectname)|%(*objectname)|%(taggername)|%(taggerdate:iso-strict)|%(contents:subject)", "refs/tags")
	if err != nil {
		return []model.TagEntry{}, nil
	}
	var tags []model.TagEntry
	for _, line := range splitGitLines(output) {
		parts := strings.SplitN(line, "|", 7)
		if len(parts) < 7 {
			continue
		}
		name, typ, sha, deref, tagger, date, msg := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]
		if deref != "" {
			sha = deref // use the commit SHA for annotated tags
		}
		tagType := "lightweight"
		if typ == "tag" {
			tagType = "annotated"
		}
		tags = append(tags, model.TagEntry{
			Name:    name,
			SHA:     sha,
			Type:    tagType,
			Message: msg,
			Tagger:  tagger,
			Date:    date,
		})
	}
	return tags, nil
}

// CreateTag creates a lightweight or annotated tag at the given ref.
func (s *RepositoryService) CreateTag(ctx context.Context, repositoryID uuid.UUID, name, ref, message string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if err := validateGitBranchSyntax(name); err != nil {
		return fmt.Errorf("invalid tag name")
	}
	if strings.TrimSpace(ref) == "" {
		ref = repo.DefaultBranch
	}
	if err := validateGitRefArgument(ref); err != nil {
		return err
	}
	if message != "" {
		return runGit(ctx, repo.StoragePath, "tag", "-a", name, ref, "-m", message)
	}
	return runGit(ctx, repo.StoragePath, "tag", name, ref)
}

// DeleteTag deletes a tag.
func (s *RepositoryService) DeleteTag(ctx context.Context, repositoryID uuid.UUID, name string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if err := validateGitBranchSyntax(name); err != nil {
		return fmt.Errorf("invalid tag name")
	}
	return runGit(ctx, repo.StoragePath, "tag", "-d", name)
}

// CreateBranch creates a new branch from the given base ref.
func (s *RepositoryService) CreateBranch(ctx context.Context, repositoryID uuid.UUID, name, from string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if err := validateGitBranchSyntax(name); err != nil {
		return err
	}
	if strings.TrimSpace(from) == "" {
		from = repo.DefaultBranch
	}
	if err := validateGitRefArgument(from); err != nil {
		return err
	}
	return runGit(ctx, repo.StoragePath, "branch", name, from)
}

// DeleteBranch deletes a branch (refuses to delete the default branch).
func (s *RepositoryService) DeleteBranch(ctx context.Context, repositoryID uuid.UUID, name string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if err := validateGitBranchSyntax(name); err != nil {
		return err
	}
	if name == repo.DefaultBranch {
		return fmt.Errorf("cannot delete the default branch")
	}
	return runGit(ctx, repo.StoragePath, "branch", "-D", name)
}

func parsePatch(patchOut string, fileStats map[string][2]int) []model.PRFileDiff {
	var diffs []model.PRFileDiff
	var current *model.PRFileDiff
	var patchLines []string

	for _, line := range strings.Split(patchOut, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				current.Patch = strings.Join(patchLines, "\n")
				diffs = append(diffs, *current)
			}
			// extract b/path
			parts := strings.Fields(line)
			path := ""
			if len(parts) >= 4 {
				path = strings.TrimPrefix(parts[3], "b/")
			}
			stats := fileStats[path]
			current = &model.PRFileDiff{Path: path, Additions: stats[0], Deletions: stats[1]}
			patchLines = nil
		} else if current != nil {
			patchLines = append(patchLines, line)
		}
	}
	if current != nil {
		current.Patch = strings.Join(patchLines, "\n")
		diffs = append(diffs, *current)
	}
	return diffs
}
