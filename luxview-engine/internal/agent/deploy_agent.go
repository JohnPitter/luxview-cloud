package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/luxview/engine/pkg/logger"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicOAuthURL   = "https://console.anthropic.com/v1/oauth/token"
	anthropicVersion    = "2023-06-01"
	defaultModel        = "claude-sonnet-4-20250514"
	defaultMaxTokens    = 4096
	defaultTemperature  = 0
	httpClientTimeout   = 60 * time.Second
)

// OAuthTokens holds OAuth token data for auto-refresh.
type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // Unix milliseconds
}

// OAuthRefreshResult contains the new tokens after a refresh.
type OAuthRefreshResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// setAuthHeader sets the appropriate auth header based on token type.
// OAuth tokens (sk-ant-oat*) use Authorization: Bearer, API keys use x-api-key.
func setAuthHeader(req *http.Request, token string) {
	if strings.HasPrefix(token, "sk-ant-oat") {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("x-api-key", token)
	}
}

// RefreshOAuthToken exchanges a refresh token for a new access token.
// Returns the new tokens or an error.
func (a *DeployAgent) RefreshOAuthToken(ctx context.Context, refreshToken string) (*OAuthRefreshResult, error) {
	log := logger.With("deploy-agent")
	log.Info().Msg("refreshing OAuth token")

	form := "grant_type=refresh_token&refresh_token=" + refreshToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicOAuthURL, strings.NewReader(form))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("OAuth refresh failed")
		return nil, fmt.Errorf("OAuth refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result OAuthRefreshResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	log.Info().Msg("OAuth token refreshed successfully")
	return &result, nil
}

// IsOAuthToken returns true if the token is an OAuth access token.
func IsOAuthToken(token string) bool {
	return strings.HasPrefix(token, "sk-ant-oat")
}

// IsTokenExpired checks if an OAuth token has expired or will expire within 60 seconds.
func IsTokenExpired(expiresAtMs int64) bool {
	if expiresAtMs == 0 {
		return false
	}
	return time.Now().UnixMilli() >= (expiresAtMs - 60000) // 60s buffer
}

// DeployAgent calls the Anthropic Messages API to analyze repositories
// and generate optimal Dockerfiles for deployment.
type DeployAgent struct {
	client *http.Client
}

// NewDeployAgent creates a new DeployAgent with a configured HTTP client.
func NewDeployAgent() *DeployAgent {
	return &DeployAgent{
		client: &http.Client{
			Timeout: httpClientTimeout,
		},
	}
}

// Analyze scans a repository and returns deployment suggestions with a generated Dockerfile.
func (a *DeployAgent) Analyze(ctx context.Context, apiKey, model, repoDir string) (*AnalysisResult, error) {
	log := logger.With("deploy-agent")
	log.Info().Str("repo", repoDir).Msg("starting repository analysis")

	userPrompt, err := BuildContext(repoDir)
	if err != nil {
		return nil, fmt.Errorf("build context: %w", err)
	}

	result, err := a.callClaude(ctx, apiKey, model, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	log.Info().Str("stack", result.Stack).Int("port", result.Port).Msg("analysis complete")
	return result, nil
}

// AnalyzeFailure diagnoses a failed build and returns a corrected Dockerfile.
func (a *DeployAgent) AnalyzeFailure(ctx context.Context, apiKey, model, repoDir, buildLog, dockerfile string) (*AnalysisResult, error) {
	log := logger.With("deploy-agent")
	log.Info().Str("repo", repoDir).Msg("starting failure analysis")

	userPrompt, err := BuildFailureContext(repoDir, buildLog, dockerfile)
	if err != nil {
		return nil, fmt.Errorf("build failure context: %w", err)
	}

	result, err := a.callClaude(ctx, apiKey, model, failureSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("analyze failure: %w", err)
	}

	log.Info().Str("stack", result.Stack).Str("diagnosis", result.Diagnosis).Msg("failure analysis complete")
	return result, nil
}

// TestConnection sends a minimal request to the Anthropic API to verify the key is valid.
// Returns the model name on success or an error describing the failure.
func (a *DeployAgent) TestConnection(ctx context.Context, apiKey, model string) (string, error) {
	if model == "" {
		model = defaultModel
	}

	reqBody := anthropicRequest{
		Model:       model,
		MaxTokens:   10,
		Temperature: 0,
		System:      "Reply with only: ok",
		Messages: []anthropicMessage{
			{Role: "user", Content: "test"},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	setAuthHeader(req, apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("invalid API key")
	}
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("API key does not have access to this model")
	}
	if resp.StatusCode != http.StatusOK {
		var apiResp anthropicResponse
		if json.Unmarshal(respBody, &apiResp) == nil && apiResp.Error != nil {
			return "", fmt.Errorf("%s: %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return model, nil
}

// anthropicRequest represents the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
}

// anthropicMessage represents a single message in the conversation.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the response from the Anthropic Messages API.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// callClaude sends a request to the Anthropic Messages API and parses the response.
func (a *DeployAgent) callClaude(ctx context.Context, apiKey, model, system, userPrompt string) (*AnalysisResult, error) {
	log := logger.With("deploy-agent")

	if model == "" {
		model = defaultModel
	}

	reqBody := anthropicRequest{
		Model:       model,
		MaxTokens:   defaultMaxTokens,
		Temperature: defaultTemperature,
		System:      system,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	setAuthHeader(req, apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	log.Debug().Str("model", model).Int("prompt_len", len(userPrompt)).Msg("calling Anthropic API")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call anthropic api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("body", string(respBody)).Msg("anthropic api error")
		return nil, fmt.Errorf("anthropic api returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("anthropic api error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	// Extract text content from the response
	var text string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	if text == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	// Strip markdown code fences if present
	text = stripCodeFences(text)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		log.Error().Str("raw_text", text).Err(err).Msg("failed to parse agent response")
		return nil, fmt.Errorf("parse agent response: %w", err)
	}

	return &result, nil
}

// stripCodeFences removes markdown code fences from a string.
// Handles ```json ... ``` and ``` ... ``` patterns.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)

	// Handle ```json or ```
	if strings.HasPrefix(s, "```") {
		// Find end of first line (the opening fence)
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
		// Remove trailing fence
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
		s = strings.TrimSpace(s)
	}

	return s
}
