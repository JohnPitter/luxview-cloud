package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type PullRequestStore interface {
	Create(ctx context.Context, pr *model.PullRequest) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.PullRequest, error)
	FindByNumber(ctx context.Context, repositoryID uuid.UUID, number int) (*model.PullRequest, error)
	List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.PullRequest, int, error)
	NextNumber(ctx context.Context, repositoryID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.PullRequestStatus, mergeCommit *string) error
	UpdateHeadSHA(ctx context.Context, id uuid.UUID, sha string) error
	CreateComment(ctx context.Context, c *model.PullRequestComment) error
	ListComments(ctx context.Context, prID uuid.UUID) ([]model.PullRequestComment, error)
	DeleteComment(ctx context.Context, commentID uuid.UUID, authorID uuid.UUID) error
}

type PullRequestService struct {
	store   PullRequestStore
	repoSvc *RepositoryService
}

func NewPullRequestService(store PullRequestStore, repoSvc *RepositoryService) *PullRequestService {
	return &PullRequestService{store: store, repoSvc: repoSvc}
}

type CreatePRRequest struct {
	RepositoryID uuid.UUID
	AuthorID     uuid.UUID
	Title        string
	Description  string
	HeadBranch   string
	BaseBranch   string
}

func (s *PullRequestService) Create(ctx context.Context, req CreatePRRequest) (*model.PullRequest, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.HeadBranch == req.BaseBranch {
		return nil, fmt.Errorf("head and base branch must be different")
	}

	repo, err := s.repoSvc.findRepository(ctx, req.RepositoryID)
	if err != nil {
		return nil, err
	}

	// Verify both branches exist
	headSHA, err := gitOutput(ctx, repo.StoragePath, "rev-parse", req.HeadBranch)
	if err != nil {
		return nil, fmt.Errorf("head branch %q not found", req.HeadBranch)
	}
	headSHA = strings.TrimSpace(headSHA)

	if _, err := gitOutput(ctx, repo.StoragePath, "rev-parse", req.BaseBranch); err != nil {
		return nil, fmt.Errorf("base branch %q not found", req.BaseBranch)
	}

	number, err := s.store.NextNumber(ctx, req.RepositoryID)
	if err != nil {
		return nil, err
	}

	pr := &model.PullRequest{
		RepositoryID: req.RepositoryID,
		AuthorID:     req.AuthorID,
		Number:       number,
		Title:        strings.TrimSpace(req.Title),
		Description:  strings.TrimSpace(req.Description),
		HeadBranch:   req.HeadBranch,
		BaseBranch:   req.BaseBranch,
		HeadSHA:      headSHA,
		Status:       model.PullRequestStatusOpen,
	}

	if err := s.store.Create(ctx, pr); err != nil {
		return nil, err
	}
	return pr, nil
}

func (s *PullRequestService) Get(ctx context.Context, repositoryID uuid.UUID, number int) (*model.PullRequest, error) {
	pr, err := s.store.FindByNumber(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if pr == nil {
		return nil, fmt.Errorf("pull request not found")
	}

	// Refresh head SHA from git (branch may have received new pushes)
	repo, err := s.repoSvc.findRepository(ctx, repositoryID)
	if err == nil {
		if sha, err := gitOutput(ctx, repo.StoragePath, "rev-parse", pr.HeadBranch); err == nil {
			sha = strings.TrimSpace(sha)
			if sha != pr.HeadSHA {
				_ = s.store.UpdateHeadSHA(ctx, pr.ID, sha)
				pr.HeadSHA = sha
			}
		}
	}
	return pr, nil
}

func (s *PullRequestService) List(ctx context.Context, repositoryID uuid.UUID, status string, limit, offset int) ([]model.PullRequest, int, error) {
	return s.store.List(ctx, repositoryID, status, limit, offset)
}

func (s *PullRequestService) Commits(ctx context.Context, repositoryID uuid.UUID, number int) ([]model.PRCommit, error) {
	pr, err := s.Get(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if pr.Status != model.PullRequestStatusOpen {
		return nil, fmt.Errorf("pull request is not open")
	}

	repo, err := s.repoSvc.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}

	// git log base..head --format="%H|%s|%an|%ai"
	out, err := gitOutput(ctx, repo.StoragePath, "log",
		fmt.Sprintf("%s..%s", pr.BaseBranch, pr.HeadBranch),
		"--format=%H|%s|%an|%ai",
	)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []model.PRCommit
	for _, line := range splitGitLines(out) {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, model.PRCommit{
			SHA:     parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits, nil
}

func (s *PullRequestService) Diff(ctx context.Context, repositoryID uuid.UUID, number int) ([]model.PRFileDiff, error) {
	pr, err := s.Get(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}

	repo, err := s.repoSvc.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}

	// Get merge base for a clean diff
	mergeBase, err := gitOutput(ctx, repo.StoragePath, "merge-base", pr.BaseBranch, pr.HeadBranch)
	if err != nil {
		mergeBase = pr.BaseBranch
	}
	mergeBase = strings.TrimSpace(mergeBase)

	// Stat: --numstat gives additions, deletions, filename
	statOut, err := gitOutput(ctx, repo.StoragePath, "diff", "--numstat", mergeBase, pr.HeadSHA)
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat failed: %w", err)
	}

	// Patch: unified diff
	patchOut, _ := gitOutput(ctx, repo.StoragePath, "diff", mergeBase, pr.HeadSHA)

	// Parse numstat into file entries
	fileStats := map[string]*model.PRFileDiff{}
	for _, line := range splitGitLines(statOut) {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		path := parts[2]
		fileStats[path] = &model.PRFileDiff{Path: path, Additions: add, Deletions: del}
	}

	// Parse unified diff into per-file patches
	currentFile := ""
	var currentPatch strings.Builder
	for _, line := range strings.Split(patchOut, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != "" {
				if f, ok := fileStats[currentFile]; ok {
					f.Patch = currentPatch.String()
				}
				currentPatch.Reset()
			}
			// Extract filename from "diff --git a/foo b/foo"
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				currentFile = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
		}
		currentPatch.WriteString(line + "\n")
	}
	if currentFile != "" {
		if f, ok := fileStats[currentFile]; ok {
			f.Patch = currentPatch.String()
		}
	}

	diffs := make([]model.PRFileDiff, 0, len(fileStats))
	for _, f := range fileStats {
		diffs = append(diffs, *f)
	}
	return diffs, nil
}

func (s *PullRequestService) Merge(ctx context.Context, repositoryID uuid.UUID, number int, userID uuid.UUID) (*model.PullRequest, error) {
	pr, err := s.Get(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if pr.Status != model.PullRequestStatusOpen {
		return nil, fmt.Errorf("pull request is not open")
	}

	repo, err := s.repoSvc.findRepository(ctx, repositoryID)
	if err != nil {
		return nil, err
	}

	// Merge in a temporary worktree to avoid touching the bare repo directly
	tmpDir, err := cloneToTemp(ctx, repo.StoragePath, pr.BaseBranch)
	if err != nil {
		return nil, fmt.Errorf("prepare merge workspace: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Fetch head branch into worktree
	if err := runGit(ctx, tmpDir, "fetch", "origin", pr.HeadBranch); err != nil {
		return nil, fmt.Errorf("fetch head branch: %w", err)
	}

	// Merge --no-ff so there is always a merge commit
	msg := fmt.Sprintf("Merge pull request #%d: %s\n\nMerge %s into %s", pr.Number, pr.Title, pr.HeadBranch, pr.BaseBranch)
	if err := runGit(ctx, tmpDir, "merge", "--no-ff", "-m", msg, "origin/"+pr.HeadBranch); err != nil {
		return nil, fmt.Errorf("merge conflict: please resolve conflicts before merging")
	}

	// Push result back to the bare repo
	if err := runGit(ctx, tmpDir, "push", "origin", pr.BaseBranch); err != nil {
		return nil, fmt.Errorf("push merge: %w", err)
	}

	// Read the merge commit SHA
	mergeCommitSHA, err := gitOutput(ctx, tmpDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read merge commit: %w", err)
	}
	mergeCommitSHA = strings.TrimSpace(mergeCommitSHA)

	if err := s.store.UpdateStatus(ctx, pr.ID, model.PullRequestStatusMerged, &mergeCommitSHA); err != nil {
		return nil, err
	}
	pr.Status = model.PullRequestStatusMerged
	pr.MergeCommit = &mergeCommitSHA
	return pr, nil
}

func (s *PullRequestService) Close(ctx context.Context, repositoryID uuid.UUID, number int, userID uuid.UUID) (*model.PullRequest, error) {
	pr, err := s.Get(ctx, repositoryID, number)
	if err != nil {
		return nil, err
	}
	if pr.AuthorID != userID {
		return nil, fmt.Errorf("forbidden")
	}
	if pr.Status != model.PullRequestStatusOpen {
		return nil, fmt.Errorf("pull request is not open")
	}
	if err := s.store.UpdateStatus(ctx, pr.ID, model.PullRequestStatusClosed, nil); err != nil {
		return nil, err
	}
	pr.Status = model.PullRequestStatusClosed
	return pr, nil
}

func (s *PullRequestService) AddComment(ctx context.Context, prID uuid.UUID, authorID uuid.UUID, body string) (*model.PullRequestComment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("comment body is required")
	}
	c := &model.PullRequestComment{
		PullRequestID: prID,
		AuthorID:      authorID,
		Body:          body,
	}
	if err := s.store.CreateComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *PullRequestService) ListComments(ctx context.Context, prID uuid.UUID) ([]model.PullRequestComment, error) {
	return s.store.ListComments(ctx, prID)
}

func (s *PullRequestService) DeleteComment(ctx context.Context, commentID uuid.UUID, authorID uuid.UUID) error {
	return s.store.DeleteComment(ctx, commentID, authorID)
}

// cloneToTemp clones the bare repo to a temporary directory and checks out branch.
func cloneToTemp(ctx context.Context, storagePath, branch string) (string, error) {
	dir, err := os.MkdirTemp("", "luxview-merge-*")
	if err != nil {
		return "", err
	}
	if err := runGit(ctx, "", "clone", "--branch", branch, storagePath, dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	// Configure identity for merge commit
	_ = runGit(ctx, dir, "config", "user.email", "ci@luxview.cloud")
	_ = runGit(ctx, dir, "config", "user.name", "LuxView")
	return dir, nil
}
