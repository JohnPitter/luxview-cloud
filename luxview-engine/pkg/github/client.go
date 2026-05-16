package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const (
	githubAPIBase    = "https://api.github.com"
	githubOAuthURL   = "https://github.com/login/oauth/access_token"
	httpTimeout      = 15 * time.Second
	jwtValidity      = 10 * time.Minute
	jwtLeeway        = 60 * time.Second
	apiVersionHeader = "X-GitHub-Api-Version"
	apiVersion       = "2022-11-28"
	acceptHeader     = "application/vnd.github+json"
)

// TokenResponse is the OAuth token exchange response.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// User represents a GitHub user profile.
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

// Repo represents a GitHub repository.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	Language      string `json:"language"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	UpdatedAt     string `json:"updated_at"`
}

// Branch represents a GitHub branch.
type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// Client wraps GitHub API calls using a user OAuth token.
type Client struct {
	httpClient *http.Client
}

// New creates a new GitHub OAuth client.
func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: httpTimeout}}
}

func (c *Client) ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (*TokenResponse, error) {
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&code=%s", githubOAuthURL, clientID, clientSecret, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("github: empty access token in response")
	}
	return &tok, nil
}

func (c *Client) GetUser(ctx context.Context, token string) (*User, error) {
	return get[User](ctx, c.httpClient, githubAPIBase+"/user", token)
}

func (c *Client) GetUserEmail(ctx context.Context, token string) (string, error) {
	type emailEntry struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	entries, err := getSlice[emailEntry](ctx, c.httpClient, githubAPIBase+"/user/emails", token)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(entries) > 0 {
		return entries[0].Email, nil
	}
	return "", nil
}

func (c *Client) ListRepos(ctx context.Context, token string, page, perPage int) ([]Repo, error) {
	url := fmt.Sprintf("%s/user/repos?sort=updated&per_page=%d&page=%d&affiliation=owner,collaborator",
		githubAPIBase, perPage, page)
	return getSlice[Repo](ctx, c.httpClient, url, token)
}

func (c *Client) ListBranches(ctx context.Context, token, owner, repo string) ([]Branch, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100", githubAPIBase, owner, repo)
	return getSlice[Branch](ctx, c.httpClient, url, token)
}

func (c *Client) GetLatestCommit(ctx context.Context, token, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", githubAPIBase, owner, repo, branch)
	type commitResp struct {
		SHA string `json:"sha"`
	}
	result, err := get[commitResp](ctx, c.httpClient, url, token)
	if err != nil {
		return "", err
	}
	return result.SHA, nil
}

func (c *Client) CreateWebhook(ctx context.Context, token, owner, repo, webhookURL, secret string) (int64, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/hooks", githubAPIBase, owner, repo)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	setAuthHeaders(req, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("github: create webhook %d: %s", resp.StatusCode, b)
	}
	var hook struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hook); err != nil {
		return 0, err
	}
	return hook.ID, nil
}

func (c *Client) DeleteWebhook(ctx context.Context, token, owner, repo string, hookID int64) error {
	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", githubAPIBase, owner, repo, hookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	setAuthHeaders(req, token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github: delete webhook %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) CreateRepo(ctx context.Context, token, name, description string, private bool) (*Repo, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"private":     private,
		"auto_init":   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAPIBase+"/user/repos", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: create repo %d: %s", resp.StatusCode, b)
	}
	var r Repo
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CommitFile creates or updates a file in the repository via the Contents API.
func (c *Client) CommitFile(ctx context.Context, token, owner, repo, path, message string, content []byte, branch string) error {
	contentsURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIBase, owner, repo, path)

	var existingSHA string
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, contentsURL, nil)
	if err != nil {
		return err
	}
	setAuthHeaders(getReq, token)
	if branch != "" {
		q := getReq.URL.Query()
		q.Set("ref", branch)
		getReq.URL.RawQuery = q.Encode()
	}
	getResp, err := c.httpClient.Do(getReq)
	if err != nil {
		return err
	}
	if getResp.StatusCode == http.StatusOK {
		var fileInfo struct {
			SHA string `json:"sha"`
		}
		if jsonErr := json.NewDecoder(getResp.Body).Decode(&fileInfo); jsonErr == nil {
			existingSHA = fileInfo.SHA
		}
	} else {
		_, _ = io.Copy(io.Discard, getResp.Body)
	}
	getResp.Body.Close()

	payload := map[string]interface{}{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	if existingSHA != "" {
		payload["sha"] = existingSHA
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, contentsURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	setAuthHeaders(putReq, token)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("github: commit file %d: %s", putResp.StatusCode, b)
	}
	return nil
}

// UpsertRepoSecret creates or updates a GitHub Actions repository secret using NaCl sealed box encryption.
func (c *Client) UpsertRepoSecret(ctx context.Context, token, owner, repo, key, value string) error {
	pubKeyURL := fmt.Sprintf("%s/repos/%s/%s/actions/public-key", githubAPIBase, owner, repo)
	pkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pubKeyURL, nil)
	if err != nil {
		return err
	}
	setAuthHeaders(pkReq, token)
	pkResp, err := c.httpClient.Do(pkReq)
	if err != nil {
		return err
	}
	defer pkResp.Body.Close()
	if pkResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(pkResp.Body)
		return fmt.Errorf("github: get public key %d: %s", pkResp.StatusCode, b)
	}
	var pubKeyResp struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	if err := json.NewDecoder(pkResp.Body).Decode(&pubKeyResp); err != nil {
		return err
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyResp.Key)
	if err != nil {
		return fmt.Errorf("github: decode public key: %w", err)
	}
	if len(pubKeyBytes) != 32 {
		return fmt.Errorf("github: unexpected public key length %d (want 32)", len(pubKeyBytes))
	}
	var recipientKey [32]byte
	copy(recipientKey[:], pubKeyBytes)

	encrypted, err := box.SealAnonymous(nil, []byte(value), &recipientKey, rand.Reader)
	if err != nil {
		return fmt.Errorf("github: seal secret: %w", err)
	}

	secretURL := fmt.Sprintf("%s/repos/%s/%s/actions/secrets/%s", githubAPIBase, owner, repo, key)
	payload := map[string]string{
		"encrypted_value": base64.StdEncoding.EncodeToString(encrypted),
		"key_id":          pubKeyResp.KeyID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, secretURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	setAuthHeaders(putReq, token)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusCreated && putResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("github: upsert secret %d: %s", putResp.StatusCode, b)
	}
	return nil
}

// AppClient authenticates as a GitHub App using JWT and installation tokens.
type AppClient struct {
	appID      int64
	privateKey *rsa.PrivateKey
	httpClient *http.Client
}

// NewAppClient creates an AppClient from a GitHub App ID and RSA private key PEM (PKCS#8 or PKCS#1).
func NewAppClient(appID int64, privateKeyPEM []byte) (*AppClient, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("github: failed to decode PEM block")
	}
	var rsaKey *rsa.PrivateKey
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		rsaKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("github: parse private key: %w", err)
		}
	} else {
		var ok bool
		rsaKey, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("github: PEM key is not RSA")
		}
	}
	return &AppClient{
		appID:      appID,
		privateKey: rsaKey,
		httpClient: &http.Client{Timeout: httpTimeout},
	}, nil
}

// generateJWT produces a signed RS256 JWT for GitHub App auth using only stdlib.
func (a *AppClient) generateJWT() (string, error) {
	now := time.Now()
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(map[string]interface{}{
		"iat": now.Add(-jwtLeeway).Unix(),
		"exp": now.Add(jwtValidity).Unix(),
		"iss": strconv.FormatInt(a.appID, 10),
	})
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(headerJSON) + "." + enc.EncodeToString(claimsJSON)

	h := sha256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, a.privateKey, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("github: sign JWT: %w", err)
	}
	return signingInput + "." + enc.EncodeToString(sig), nil
}

// GetInstallationToken exchanges an installation ID for a short-lived token (valid ~1h).
func (a *AppClient) GetInstallationToken(ctx context.Context, installationID int64) (string, time.Time, error) {
	jwt, err := a.generateJWT()
	if err != nil {
		return "", time.Time{}, err
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", githubAPIBase, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set(apiVersionHeader, apiVersion)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("github: get installation token %d: %s", resp.StatusCode, b)
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, err
	}
	if result.Token == "" {
		return "", time.Time{}, fmt.Errorf("github: empty installation token")
	}
	return result.Token, result.ExpiresAt, nil
}

// InstallURL returns the GitHub App installation URL for the given app slug.
func (a *AppClient) InstallURL(appSlug string) string {
	return fmt.Sprintf("https://github.com/apps/%s/installations/new", appSlug)
}

func setAuthHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set(apiVersionHeader, apiVersion)
}

func get[T any](ctx context.Context, client *http.Client, url, token string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: GET %s %d: %s", url, resp.StatusCode, b)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func getSlice[T any](ctx context.Context, client *http.Client, url, token string) ([]T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: GET %s %d: %s", url, resp.StatusCode, b)
	}
	var result []T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
