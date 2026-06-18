package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

const (
	defaultRepositoryBranch  = "main"
	defaultRepositoryBaseDir = "/data/luxview/repositories"
	gitDirectoryMode         = 0755
)

var repositorySlugRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,118}[a-z0-9])?$`)

type RepositoryStore interface {
	Create(ctx context.Context, repo *model.Repository) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Repository, error)
	FindByUserAndSlug(ctx context.Context, userID uuid.UUID, slug string) (*model.Repository, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CreateRemote(ctx context.Context, remote *model.RepositoryRemote) error
	ListRemotes(ctx context.Context, repositoryID uuid.UUID) ([]model.RepositoryRemote, error)
	UpdateRemoteSyncStatus(ctx context.Context, remoteID uuid.UUID, status model.RepositorySyncStatus, errMsg string) error
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
	return &RepositoryService{store: store, baseDir: baseDir}
}

// WithBackupSupport attaches a token provider and user lookup so SyncBackup can authenticate.
func (s *RepositoryService) WithBackupSupport(tokenProvider BackupTokenProvider, userLookup UserLookup) {
	s.tokenProvider = tokenProvider
	s.userLookup = userLookup
}

func (s *RepositoryService) Create(ctx context.Context, req CreateRepositoryRequest) (*model.Repository, error) {
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
	if err := s.store.Delete(ctx, repositoryID); err != nil {
		return err
	}
	_ = os.RemoveAll(repo.StoragePath)
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
	commit, err := gitOutput(ctx, repo.StoragePath, "rev-parse", normalizedRef+"^{commit}")
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

// SyncBackup pushes all refs to the backup remote.
// userID is the repository owner — used to retrieve the GitHub token.
// Failure is non-fatal by design: it updates the sync status but does not propagate the error.
func (s *RepositoryService) SyncBackup(ctx context.Context, repositoryID uuid.UUID, remoteID uuid.UUID, userID uuid.UUID) error {
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
	// Embed token: https://github.com/... → https://<token>@github.com/...
	withToken := strings.Replace(remoteURL, "https://", fmt.Sprintf("https://%s@", token), 1)
	return withToken, nil
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
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
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
		return "", fmt.Errorf("git %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
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
	if strings.TrimSpace(ref) == "" {
		ref = repo.DefaultBranch
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
	return runGit(ctx, repo.StoragePath, "tag", "-d", name)
}

// CreateBranch creates a new branch from the given base ref.
func (s *RepositoryService) CreateBranch(ctx context.Context, repositoryID uuid.UUID, name, from string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(from) == "" {
		from = repo.DefaultBranch
	}
	return runGit(ctx, repo.StoragePath, "branch", name, from)
}

// DeleteBranch deletes a branch (refuses to delete the default branch).
func (s *RepositoryService) DeleteBranch(ctx context.Context, repositoryID uuid.UUID, name string) error {
	repo, err := s.findRepository(ctx, repositoryID)
	if err != nil {
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
