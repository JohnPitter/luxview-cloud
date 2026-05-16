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
}

type RepositoryService struct {
	store   RepositoryStore
	baseDir string
}

type CreateRepositoryRequest struct {
	UserID        uuid.UUID
	Name          string
	Slug          string
	DefaultBranch string
	Visibility    model.RepositoryVisibility
}

func NewRepositoryService(store RepositoryStore, baseDir string) *RepositoryService {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = defaultRepositoryBaseDir
	}
	return &RepositoryService{store: store, baseDir: baseDir}
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
