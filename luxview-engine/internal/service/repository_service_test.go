package service

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type fakeRepositoryStore struct {
	repos   map[uuid.UUID]*model.Repository
	remotes []model.RepositoryRemote
}

func newFakeRepositoryStore() *fakeRepositoryStore {
	return &fakeRepositoryStore{repos: make(map[uuid.UUID]*model.Repository)}
}

func (s *fakeRepositoryStore) Create(_ context.Context, repo *model.Repository) error {
	copy := *repo
	s.repos[repo.ID] = &copy
	return nil
}

func (s *fakeRepositoryStore) FindByID(_ context.Context, id uuid.UUID) (*model.Repository, error) {
	repo := s.repos[id]
	if repo == nil {
		return nil, nil
	}
	copy := *repo
	return &copy, nil
}

func (s *fakeRepositoryStore) FindByUserAndSlug(_ context.Context, userID uuid.UUID, slug string) (*model.Repository, error) {
	for _, r := range s.repos {
		if r.UserID == userID && r.Slug == slug {
			copy := *r
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *fakeRepositoryStore) Delete(_ context.Context, id uuid.UUID) error {
	delete(s.repos, id)
	return nil
}

func (s *fakeRepositoryStore) CreateRemote(_ context.Context, remote *model.RepositoryRemote) error {
	remote.ID = uuid.New()
	s.remotes = append(s.remotes, *remote)
	return nil
}

func (s *fakeRepositoryStore) ListRemotes(_ context.Context, repositoryID uuid.UUID) ([]model.RepositoryRemote, error) {
	var result []model.RepositoryRemote
	for _, r := range s.remotes {
		if r.RepositoryID == repositoryID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *fakeRepositoryStore) UpdateRemoteSyncStatus(_ context.Context, remoteID uuid.UUID, status model.RepositorySyncStatus, errMsg string) error {
	for i, r := range s.remotes {
		if r.ID == remoteID {
			s.remotes[i].LastSyncStatus = &status
			s.remotes[i].LastSyncError = errMsg
			return nil
		}
	}
	return nil
}

func (s *fakeRepositoryStore) MarkBackupsPending(_ context.Context, repositoryID uuid.UUID) error {
	for i, remote := range s.remotes {
		if remote.RepositoryID != repositoryID {
			continue
		}
		status := model.RepositorySyncStatusPending
		s.remotes[i].LastSyncStatus = &status
		s.remotes[i].LastSyncError = ""
	}
	return nil
}

func TestRepositoryServiceCreateInitializesBareRepo(t *testing.T) {
	ctx := context.Background()
	store := newFakeRepositoryStore()
	svc := NewRepositoryService(store, t.TempDir())
	userID := uuid.New()

	repo, err := svc.Create(ctx, CreateRepositoryRequest{
		UserID: userID,
		Name:   "My App",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if repo.UserID != userID {
		t.Fatalf("UserID = %s, want %s", repo.UserID, userID)
	}
	if repo.Slug != "my-app" {
		t.Fatalf("Slug = %q, want my-app", repo.Slug)
	}
	if repo.DefaultBranch != defaultRepositoryBranch {
		t.Fatalf("DefaultBranch = %q, want %q", repo.DefaultBranch, defaultRepositoryBranch)
	}
	if repo.Visibility != model.RepositoryVisibilityPrivate {
		t.Fatalf("Visibility = %q, want %q", repo.Visibility, model.RepositoryVisibilityPrivate)
	}
	if _, err := os.Stat(filepath.Join(repo.StoragePath, "HEAD")); err != nil {
		t.Fatalf("bare repo HEAD not created: %v", err)
	}
	if _, err := store.FindByID(ctx, repo.ID); err != nil {
		t.Fatalf("FindByID() after create error = %v", err)
	}
}

func TestRepositoryServiceListBranchesResolveRefAndCheckout(t *testing.T) {
	ctx := context.Background()
	store := newFakeRepositoryStore()
	svc := NewRepositoryService(store, t.TempDir())
	repo, err := svc.Create(ctx, CreateRepositoryRequest{
		UserID: uuid.New(),
		Name:   "service-api",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	seedRepository(t, ctx, repo.StoragePath)

	branches, err := svc.ListBranches(ctx, repo.ID)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	wantBranches := []string{"feature-x", "main"}
	if !reflect.DeepEqual(branches, wantBranches) {
		t.Fatalf("ListBranches() = %#v, want %#v", branches, wantBranches)
	}

	mainCommit, err := svc.ResolveRef(ctx, repo.ID, "main")
	if err != nil {
		t.Fatalf("ResolveRef(main) error = %v", err)
	}
	featureCommit, err := svc.ResolveRef(ctx, repo.ID, "feature-x")
	if err != nil {
		t.Fatalf("ResolveRef(feature-x) error = %v", err)
	}
	if mainCommit == "" || featureCommit == "" || mainCommit == featureCommit {
		t.Fatalf("unexpected commits: main=%q feature=%q", mainCommit, featureCommit)
	}

	checkoutDir := filepath.Join(t.TempDir(), "checkout")
	result, err := svc.Checkout(ctx, repo.ID, "feature-x", checkoutDir)
	if err != nil {
		t.Fatalf("Checkout() error = %v", err)
	}
	if result.CommitSHA != featureCommit {
		t.Fatalf("Checkout commit = %q, want %q", result.CommitSHA, featureCommit)
	}
	content, err := os.ReadFile(filepath.Join(checkoutDir, "README.md"))
	if err != nil {
		t.Fatalf("read checkout file: %v", err)
	}
	normalizedContent := strings.ReplaceAll(string(content), "\r\n", "\n")
	if normalizedContent != "feature\n" {
		t.Fatalf("checkout README = %q, want feature", string(content))
	}
}

func TestRepositoryServiceReceiveHookEnforcesProtectedBranches(t *testing.T) {
	ctx := context.Background()
	store := newFakeRepositoryStore()
	baseDir, err := os.MkdirTemp("", "lvh-")
	if err != nil {
		t.Fatalf("create short temp directory: %v", err)
	}
	defer os.RemoveAll(baseDir)
	svc := NewRepositoryService(store, baseDir)
	repo, err := svc.Create(ctx, CreateRepositoryRequest{UserID: uuid.New(), Name: "protected-repo"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	seedRepository(t, ctx, repo.StoragePath)

	rule := []model.BranchProtectionRule{{
		Branch:              "main",
		RequireReviews:      true,
		RequiredApprovals:   1,
		RequireStatusChecks: true,
		BlockForcePush:      true,
	}}
	if err := svc.EnsureReceiveHook(ctx, repo.ID, rule); err != nil {
		t.Fatalf("EnsureReceiveHook() error = %v", err)
	}
	if err := svc.EnsureReceiveHook(ctx, repo.ID, rule); err != nil {
		t.Fatalf("EnsureReceiveHook() second call error = %v", err)
	}

	worktree, err := os.MkdirTemp("", "lvw-")
	if err != nil {
		t.Fatalf("create short worktree: %v", err)
	}
	defer os.RemoveAll(worktree)
	if err := runGit(ctx, "", "clone", repo.StoragePath, worktree); err != nil {
		t.Fatalf("clone protected repository: %v", err)
	}
	if err := runGit(ctx, worktree, "config", "user.email", "test@luxview.local"); err != nil {
		t.Fatalf("configure email: %v", err)
	}
	if err := runGit(ctx, worktree, "config", "user.name", "LuxView Test"); err != nil {
		t.Fatalf("configure name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "protected.txt"), []byte("blocked\n"), 0644); err != nil {
		t.Fatalf("write protected file: %v", err)
	}
	if err := runGit(ctx, worktree, "add", "protected.txt"); err != nil {
		t.Fatalf("stage protected file: %v", err)
	}
	if err := runGit(ctx, worktree, "commit", "-m", "direct push"); err != nil {
		t.Fatalf("commit protected file: %v", err)
	}
	if err := runGit(ctx, worktree, "push", "origin", "main"); err == nil {
		t.Fatal("expected direct push to protected branch to fail")
	}
	if err := runGitWithEnv(ctx, worktree, map[string]string{"LUXVIEW_INTERNAL_MERGE": "1"}, "push", "origin", "main"); err != nil {
		t.Fatalf("internal merge push: %v", err)
	}
}

func TestRepositoryServiceRedactsRemoteCredentials(t *testing.T) {
	redacted := redactGitOutput("fatal: unable to access https://ghp_secret@github.com/acme/repo.git")
	if strings.Contains(redacted, "ghp_secret") || !strings.Contains(redacted, "https://***@github.com") {
		t.Fatalf("redacted output = %q", redacted)
	}
}

func TestGitObjectStorageBytes(t *testing.T) {
	output := "count: 2\nsize: 4\nin-pack: 1\npacks: 1\nsize-pack: 8\nsize-garbage: 1\n"
	if got, want := gitObjectStorageBytes(output), int64(13*1024); got != want {
		t.Fatalf("gitObjectStorageBytes() = %d, want %d", got, want)
	}
}

func seedRepository(t *testing.T, ctx context.Context, barePath string) {
	t.Helper()
	worktree := t.TempDir()
	if err := runGit(ctx, "", "init", "--initial-branch", defaultRepositoryBranch, worktree); err != nil {
		t.Fatalf("git init seed: %v", err)
	}
	if err := runGit(ctx, worktree, "config", "user.email", "test@luxview.local"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if err := runGit(ctx, worktree, "config", "user.name", "LuxView Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("main\n"), 0644); err != nil {
		t.Fatalf("write main readme: %v", err)
	}
	if err := runGit(ctx, worktree, "add", "README.md"); err != nil {
		t.Fatalf("git add main: %v", err)
	}
	if err := runGit(ctx, worktree, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit main: %v", err)
	}
	if err := runGit(ctx, worktree, "remote", "add", "origin", barePath); err != nil {
		t.Fatalf("git remote add: %v", err)
	}
	if err := runGit(ctx, worktree, "push", "origin", defaultRepositoryBranch); err != nil {
		t.Fatalf("git push main: %v", err)
	}
	if err := runGit(ctx, worktree, "checkout", "-b", "feature-x"); err != nil {
		t.Fatalf("git checkout feature: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("write feature readme: %v", err)
	}
	if err := runGit(ctx, worktree, "commit", "-am", "feature commit"); err != nil {
		t.Fatalf("git commit feature: %v", err)
	}
	if err := runGit(ctx, worktree, "push", "origin", "feature-x"); err != nil {
		t.Fatalf("git push feature: %v", err)
	}
}
