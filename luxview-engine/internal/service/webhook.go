package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// WebhookService processes GitHub webhook events.
type WebhookService struct {
	appRepo    *repository.AppRepo
	deployer   *Deployer
	buildQueue chan<- DeployRequest
}

func NewWebhookService(appRepo *repository.AppRepo, buildQueue chan<- DeployRequest) *WebhookService {
	return &WebhookService{
		appRepo:    appRepo,
		buildQueue: buildQueue,
	}
}

// GitHubPushEvent represents a GitHub push webhook payload.
type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"head_commit"`
	Sender struct {
		ID int64 `json:"id"`
	} `json:"sender"`
}

// ProcessPush handles a GitHub push event.
func (ws *WebhookService) ProcessPush(ctx context.Context, payload []byte) error {
	log := logger.With("webhook")

	var event GitHubPushEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("unmarshal push event: %w", err)
	}

	// Extract branch name from ref (refs/heads/main -> main)
	branch := strings.TrimPrefix(event.Ref, "refs/heads/")

	repoURL := event.Repository.CloneURL
	if repoURL == "" {
		repoURL = event.Repository.HTMLURL + ".git"
	}

	log.Info().
		Str("repo", event.Repository.FullName).
		Str("branch", branch).
		Str("commit", event.HeadCommit.ID[:min(7, len(event.HeadCommit.ID))]).
		Int64("sender_id", event.Sender.ID).
		Msg("push event received")

	// Find all apps that match this repo + branch with auto_deploy enabled
	apps, _, err := ws.appRepo.ListAll(ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("list apps: %w", err)
	}

	log.Debug().Int("total_apps", len(apps)).Msg("checking apps for auto-deploy match")

	matched := 0
	for _, app := range apps {
		if !app.AutoDeploy {
			log.Debug().Str("app", app.Subdomain).Msg("skipping app: auto_deploy disabled")
			continue
		}
		repoMatch := matchesRepo(app.RepoURL, repoURL)
		log.Debug().Str("app", app.Subdomain).Str("app_repo", app.RepoURL).Bool("repo_match", repoMatch).Msg("checking repo match")
		if !repoMatch {
			log.Debug().Str("app", app.Subdomain).Msg("skipping app: repo mismatch")
			continue
		}
		if app.RepoBranch != branch {
			log.Debug().Str("app", app.Subdomain).Str("app_branch", app.RepoBranch).Str("event_branch", branch).Msg("skipping app: branch mismatch")
			continue
		}

		req := DeployRequest{
			AppID:     app.ID,
			UserID:    app.UserID,
			CommitSHA: event.HeadCommit.ID,
			CommitMsg: event.HeadCommit.Message,
			Source:    "auto",
		}

		select {
		case ws.buildQueue <- req:
			matched++
			log.Info().Str("app", app.Subdomain).Msg("deploy queued from webhook")
		default:
			log.Warn().Str("app", app.Subdomain).Msg("build queue full, skipping")
		}
	}

	log.Info().Int("matched", matched).Msg("push event processed")
	return nil
}

// VerifySignature verifies the GitHub webhook signature.
func VerifySignature(payload []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}

	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// matchesRepo compares two repo URLs ignoring protocol and .git suffix differences.
func matchesRepo(a, b string) bool {
	return normalizeRepoURL(a) == normalizeRepoURL(b)
}

func normalizeRepoURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")
	url = strings.ReplaceAll(url, ":", "/")
	return strings.ToLower(url)
}
