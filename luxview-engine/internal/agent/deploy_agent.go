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
	openRouterAPIURL   = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel       = "anthropic/claude-sonnet-4"
	defaultMaxTokens   = 8192
	defaultTemperature = 0
	httpClientTimeout  = 60 * time.Second
)

// DeployAgent calls the OpenRouter API to analyze repositories
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

	result, err := a.callLLM(ctx, apiKey, model, systemPrompt, userPrompt)
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

	result, err := a.callLLM(ctx, apiKey, model, failureSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("analyze failure: %w", err)
	}

	log.Info().Str("stack", result.Stack).Str("diagnosis", result.Diagnosis).Msg("failure analysis complete")
	return result, nil
}

// TestConnection sends a minimal request to the OpenRouter API to verify the key is valid.
// Returns the model name on success or an error describing the failure.
func (a *DeployAgent) TestConnection(ctx context.Context, apiKey, model string) (string, error) {
	if model == "" {
		model = defaultModel
	}

	reqBody := openRouterRequest{
		Model:       model,
		MaxTokens:   10,
		Temperature: 0,
		Messages: []openRouterMessage{
			{Role: "system", Content: "Reply with only: ok"},
			{Role: "user", Content: "test"},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp openRouterResponse
		if json.Unmarshal(respBody, &apiResp) == nil && apiResp.Error != nil {
			return "", fmt.Errorf("%s", apiResp.Error.Message)
		}
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return model, nil
}

// openRouterRequest represents the request body for the OpenRouter chat completions API.
type openRouterRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature float64              `json:"temperature"`
	Messages    []openRouterMessage  `json:"messages"`
}

// openRouterMessage represents a single message in the conversation.
type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openRouterResponse represents the response from the OpenRouter chat completions API.
type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// callLLM sends a request to the OpenRouter API and parses the response.
func (a *DeployAgent) callLLM(ctx context.Context, apiKey, model, system, userPrompt string) (*AnalysisResult, error) {
	log := logger.With("deploy-agent")

	if model == "" {
		model = defaultModel
	}

	reqBody := openRouterRequest{
		Model:       model,
		MaxTokens:   defaultMaxTokens,
		Temperature: defaultTemperature,
		Messages: []openRouterMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	log.Debug().Str("model", model).Int("prompt_len", len(userPrompt)).Msg("calling OpenRouter API")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("body", string(respBody)).Msg("OpenRouter API error")
		return nil, fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp openRouterResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenRouter API error: %s", apiResp.Error.Message)
	}

	// Extract text content from the response
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	text := apiResp.Choices[0].Message.Content

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
