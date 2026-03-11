package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubClient wraps GitHub API calls.
type GitHubClient struct {
	client *http.Client
}

func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GitHubRepo represents a GitHub repository.
type GitHubRepo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Language      string `json:"language"`
	UpdatedAt     string `json:"updated_at"`
}

// GitHubBranch represents a GitHub branch.
type GitHubBranch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// GitHubUser represents a GitHub user profile.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// GitHubTokenResponse is the OAuth token exchange response.
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// ExchangeCode exchanges an OAuth code for an access token.
func (g *GitHubClient) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (*GitHubTokenResponse, error) {
	url := fmt.Sprintf("https://github.com/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s",
		clientID, clientSecret, code)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access token from GitHub")
	}
	return &tokenResp, nil
}

// GetUser fetches the authenticated user's profile.
func (g *GitHubClient) GetUser(ctx context.Context, token string) (*GitHubUser, error) {
	return doGitHubGet[GitHubUser](ctx, g.client, "https://api.github.com/user", token)
}

// GetUserEmail fetches the primary email if not available in profile.
func (g *GitHubClient) GetUserEmail(ctx context.Context, token string) (string, error) {
	type emailResp struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}

	emails, err := doGitHubGetSlice[emailResp](ctx, g.client, "https://api.github.com/user/emails", token)
	if err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}

// ListRepos lists the authenticated user's repositories.
func (g *GitHubClient) ListRepos(ctx context.Context, token string, page, perPage int) ([]GitHubRepo, error) {
	url := fmt.Sprintf("https://api.github.com/user/repos?sort=updated&per_page=%d&page=%d&affiliation=owner,collaborator", perPage, page)
	return doGitHubGetSlice[GitHubRepo](ctx, g.client, url, token)
}

// ListBranches lists branches for a repository.
func (g *GitHubClient) ListBranches(ctx context.Context, token, owner, repo string) ([]GitHubBranch, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", owner, repo)
	return doGitHubGetSlice[GitHubBranch](ctx, g.client, url, token)
}

// GetLatestCommit gets the latest commit SHA for a branch.
func (g *GitHubClient) GetLatestCommit(ctx context.Context, token, owner, repo, branch string) (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, branch)

	type commitResp struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}

	result, err := doGitHubGet[commitResp](ctx, g.client, url, token)
	if err != nil {
		return "", "", err
	}
	return result.SHA, result.Commit.Message, nil
}

// GetFileContent gets a file's content and SHA from a repository.
func (g *GitHubClient) GetFileContent(ctx context.Context, token, owner, repo, path, branch string) (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)

	type fileResp struct {
		Content string `json:"content"`
		SHA     string `json:"sha"`
	}

	result, err := doGitHubGet[fileResp](ctx, g.client, url, token)
	if err != nil {
		return "", "", err
	}

	// GitHub returns base64 content with newlines; strip them before decoding
	cleaned := strings.ReplaceAll(result.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return string(decoded), result.SHA, nil
}

// CreateBranch creates a new branch from a commit SHA.
func (g *GitHubClient) CreateBranch(ctx context.Context, token, owner, repo, branchName, fromSHA string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", owner, repo)

	body, err := json.Marshal(map[string]string{
		"ref": "refs/heads/" + branchName,
		"sha": fromSHA,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error %d creating branch: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateOrUpdateFile creates or updates a file on a branch via the Contents API.
// For updates, sha must be the current file's SHA. For creates, sha should be empty.
// The content parameter must be base64 encoded.
func (g *GitHubClient) CreateOrUpdateFile(ctx context.Context, token, owner, repo, path, message, content, sha, branch string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	payload := map[string]string{
		"message": message,
		"content": content,
		"branch":  branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error %d creating/updating file: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreatePullRequest creates a pull request and returns the HTML URL.
func (g *GitHubClient) CreatePullRequest(ctx context.Context, token, owner, repo, title, body, head, base string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)

	payload, err := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github API error %d creating pull request: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.HTMLURL, nil
}

// GitHubWebhook represents a GitHub repository webhook.
type GitHubWebhook struct {
	ID     int64  `json:"id"`
	Active bool   `json:"active"`
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
}

// CreateWebhook creates a push webhook on a GitHub repository.
func (g *GitHubClient) CreateWebhook(ctx context.Context, token, owner, repo, webhookURL, secret string) (int64, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repo)

	payload := map[string]interface{}{
		"name":   "web",
		"active": true,
		"events": []string{"push"},
		"config": map[string]string{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("github API error %d creating webhook: %s", resp.StatusCode, string(respBody))
	}

	var hook GitHubWebhook
	if err := json.NewDecoder(resp.Body).Decode(&hook); err != nil {
		return 0, err
	}
	return hook.ID, nil
}

// DeleteWebhook removes a webhook from a GitHub repository.
func (g *GitHubClient) DeleteWebhook(ctx context.Context, token, owner, repo string, hookID int64) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%d", owner, repo, hookID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error %d deleting webhook: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func doGitHubGet[T any](ctx context.Context, client *http.Client, url, token string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func doGitHubGetSlice[T any](ctx context.Context, client *http.Client, url, token string) ([]T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	var result []T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
