package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/crypto"
	pkggithub "github.com/luxview/engine/pkg/github"
	"github.com/luxview/engine/pkg/logger"
)

const tokenExpiryBuffer = 5 * time.Minute

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// GitHubAppService manages GitHub App installation tokens and high-level repo operations.
type GitHubAppService struct {
	appClient     *pkggithub.AppClient
	oauthClient   *pkggithub.Client
	userRepo      *repository.UserRepo
	encryptionKey []byte

	mu     sync.Mutex
	tokens map[int64]cachedToken // installation_id → token
}

func NewGitHubAppService(appClient *pkggithub.AppClient, userRepo *repository.UserRepo, encryptionKey []byte) *GitHubAppService {
	return &GitHubAppService{
		appClient:     appClient,
		oauthClient:   pkggithub.New(),
		userRepo:      userRepo,
		encryptionKey: encryptionKey,
		tokens:        make(map[int64]cachedToken),
	}
}

// TokenForUser returns a valid GitHub token for the given user — installation token if the
// GitHub App is installed, falling back to the stored OAuth token.
func (s *GitHubAppService) TokenForUser(ctx context.Context, user *model.User) (string, error) {
	if user.InstallationID != 0 && s.appClient != nil {
		return s.installationToken(ctx, user.InstallationID)
	}
	if user.GitHubToken == "" {
		return "", fmt.Errorf("user has no GitHub token or App installation")
	}
	token, err := crypto.Decrypt(user.GitHubToken, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("decrypt github token: %w", err)
	}
	return token, nil
}

func (s *GitHubAppService) installationToken(ctx context.Context, installationID int64) (string, error) {
	s.mu.Lock()
	if cached, ok := s.tokens[installationID]; ok && time.Until(cached.expiresAt) > tokenExpiryBuffer {
		s.mu.Unlock()
		return cached.token, nil
	}
	s.mu.Unlock()

	token, expiresAt, err := s.appClient.GetInstallationToken(ctx, installationID)
	if err != nil {
		return "", fmt.Errorf("get installation token: %w", err)
	}

	s.mu.Lock()
	s.tokens[installationID] = cachedToken{token: token, expiresAt: expiresAt}
	s.mu.Unlock()

	return token, nil
}

// HandleInstallation processes a GitHub App installation event and links it to a user.
func (s *GitHubAppService) HandleInstallation(ctx context.Context, installationID int64, senderGitHubID int64) error {
	log := logger.With("github-app")
	user, err := s.userRepo.FindByGitHubID(ctx, senderGitHubID)
	if err != nil {
		return fmt.Errorf("find user by github_id %d: %w", senderGitHubID, err)
	}
	if user == nil {
		log.Warn().Int64("github_id", senderGitHubID).Msg("installation event for unknown user — skipping")
		return nil
	}
	if err := s.userRepo.UpdateInstallationID(ctx, user.ID, installationID); err != nil {
		return err
	}
	log.Info().Str("user", user.Username).Int64("installation_id", installationID).Msg("GitHub App installed")
	return nil
}

// HandleUninstallation clears the installation ID when the App is removed.
func (s *GitHubAppService) HandleUninstallation(ctx context.Context, installationID int64) error {
	log := logger.With("github-app")
	user, err := s.userRepo.FindByInstallationID(ctx, installationID)
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}
	if err := s.userRepo.UpdateInstallationID(ctx, user.ID, 0); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.tokens, installationID)
	s.mu.Unlock()

	log.Info().Str("user", user.Username).Msg("GitHub App uninstalled")
	return nil
}

// CompleteInstallation is called after the OAuth redirect that carries both code and installation_id.
// It links the installation to the logged-in user.
func (s *GitHubAppService) CompleteInstallation(ctx context.Context, userID uuid.UUID, installationID int64) error {
	return s.userRepo.UpdateInstallationID(ctx, userID, installationID)
}

type CreateRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

type CreateRepoResponse struct {
	HTMLURL       string `json:"html_url"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

// CreateRepo creates a new GitHub repository for the user.
func (s *GitHubAppService) CreateRepo(ctx context.Context, user *model.User, req CreateRepoRequest) (*CreateRepoResponse, error) {
	token, err := s.TokenForUser(ctx, user)
	if err != nil {
		return nil, err
	}
	repo, err := s.oauthClient.CreateRepo(ctx, token, req.Name, req.Description, req.Private)
	if err != nil {
		return nil, err
	}
	return &CreateRepoResponse{
		HTMLURL:       repo.HTMLURL,
		FullName:      repo.FullName,
		DefaultBranch: repo.DefaultBranch,
		CloneURL:      "https://github.com/" + repo.FullName + ".git",
	}, nil
}

type CommitWorkflowRequest struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Branch       string `json:"branch"`
	WorkflowName string `json:"workflow_name"`
	Content      string `json:"content"`
}

// CommitWorkflow writes a workflow YAML file to `.github/workflows/{name}.yml` in the repo.
func (s *GitHubAppService) CommitWorkflow(ctx context.Context, user *model.User, req CommitWorkflowRequest) error {
	token, err := s.TokenForUser(ctx, user)
	if err != nil {
		return err
	}
	name := req.WorkflowName
	if name == "" {
		name = "ci"
	}
	if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
		name += ".yml"
	}
	path := ".github/workflows/" + name
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	return s.oauthClient.CommitFile(ctx, token, req.Owner, req.Repo, path,
		"ci: add LuxView workflow "+name, []byte(req.Content), branch)
}

type SyncSecretsRequest struct {
	Owner  string            `json:"owner"`
	Repo   string            `json:"repo"`
	Secrets map[string]string `json:"secrets"` // key → plaintext value
}

// SyncSecretsToGitHub upserts a map of secrets as GitHub Actions repository secrets.
func (s *GitHubAppService) SyncSecretsToGitHub(ctx context.Context, user *model.User, req SyncSecretsRequest) error {
	token, err := s.TokenForUser(ctx, user)
	if err != nil {
		return err
	}
	for key, value := range req.Secrets {
		if err := s.oauthClient.UpsertRepoSecret(ctx, token, req.Owner, req.Repo, key, value); err != nil {
			return fmt.Errorf("sync secret %q: %w", key, err)
		}
	}
	return nil
}
