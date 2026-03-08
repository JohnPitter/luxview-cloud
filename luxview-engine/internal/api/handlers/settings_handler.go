package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/luxview/engine/internal/agent"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
}

func NewSettingsHandler(settingsRepo *repository.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: settingsRepo}
}

// aiSettingsResponse is the JSON shape returned by GetAISettings.
type aiSettingsResponse struct {
	AuthMode           string `json:"authMode"` // "api_key" or "oauth"
	AnthropicAPIKey    string `json:"anthropicApiKey"`
	OAuthAccessToken   string `json:"oauthAccessToken"`
	OAuthRefreshToken  string `json:"oauthRefreshToken"`
	OAuthExpiresAt     string `json:"oauthExpiresAt"`
	ClaudeClientID     string `json:"claudeClientId"`
	ClaudeClientSecret string `json:"claudeClientSecret"`
	AIEnabled          bool   `json:"aiEnabled"`
	AIModel            string `json:"aiModel"`
}

// updateAISettingsRequest accepts partial updates (pointer fields).
type updateAISettingsRequest struct {
	AuthMode           *string `json:"authMode"`
	AnthropicAPIKey    *string `json:"anthropicApiKey"`
	OAuthAccessToken   *string `json:"oauthAccessToken"`
	OAuthRefreshToken  *string `json:"oauthRefreshToken"`
	OAuthExpiresAt     *string `json:"oauthExpiresAt"`
	ClaudeClientID     *string `json:"claudeClientId"`
	ClaudeClientSecret *string `json:"claudeClientSecret"`
	AIEnabled          *bool   `json:"aiEnabled"`
	AIModel            *string `json:"aiModel"`
}

// maskSecret masks a string showing only the first 4 and last 4 characters.
// If the string is too short (<=8), it returns a fixed mask.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// GetAISettings returns all AI-related settings with secrets masked.
func (h *SettingsHandler) GetAISettings(w http.ResponseWriter, r *http.Request) {
	log := logger.With("settings")
	ctx := r.Context()

	settings, err := h.settingsRepo.GetAll(ctx, "ai_")
	if err != nil {
		log.Error().Err(err).Msg("failed to get AI settings")
		writeError(w, http.StatusInternalServerError, "failed to get AI settings")
		return
	}

	authMode := settings["auth_mode"]
	if authMode == "" {
		// Auto-detect from existing data
		if settings["anthropic_api_key"] != "" {
			authMode = "api_key"
		} else if settings["oauth_access_token"] != "" {
			authMode = "oauth"
		} else {
			authMode = "api_key"
		}
	}

	resp := aiSettingsResponse{
		AuthMode:           authMode,
		AnthropicAPIKey:    maskSecret(settings["anthropic_api_key"]),
		OAuthAccessToken:   maskSecret(settings["oauth_access_token"]),
		OAuthRefreshToken:  maskSecret(settings["oauth_refresh_token"]),
		OAuthExpiresAt:     settings["oauth_expires_at"],
		ClaudeClientID:     settings["claude_client_id"],
		ClaudeClientSecret: maskSecret(settings["claude_client_secret"]),
		AIEnabled:          settings["enabled"] == "true",
		AIModel:            settings["model"],
	}

	if resp.AIModel == "" {
		resp.AIModel = "claude-sonnet-4-20250514"
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateAISettings updates AI settings. Only provided fields are updated.
func (h *SettingsHandler) UpdateAISettings(w http.ResponseWriter, r *http.Request) {
	log := logger.With("settings")
	ctx := r.Context()

	var req updateAISettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AuthMode != nil {
		if err := h.settingsRepo.Set(ctx, "ai_auth_mode", *req.AuthMode, false); err != nil {
			log.Error().Err(err).Msg("failed to set auth mode")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.AnthropicAPIKey != nil {
		if err := h.settingsRepo.Set(ctx, "ai_anthropic_api_key", *req.AnthropicAPIKey, true); err != nil {
			log.Error().Err(err).Msg("failed to set anthropic api key")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.OAuthAccessToken != nil {
		if err := h.settingsRepo.Set(ctx, "ai_oauth_access_token", *req.OAuthAccessToken, true); err != nil {
			log.Error().Err(err).Msg("failed to set oauth access token")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.OAuthRefreshToken != nil {
		if err := h.settingsRepo.Set(ctx, "ai_oauth_refresh_token", *req.OAuthRefreshToken, true); err != nil {
			log.Error().Err(err).Msg("failed to set oauth refresh token")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.OAuthExpiresAt != nil {
		if err := h.settingsRepo.Set(ctx, "ai_oauth_expires_at", *req.OAuthExpiresAt, false); err != nil {
			log.Error().Err(err).Msg("failed to set oauth expires at")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.ClaudeClientID != nil {
		if err := h.settingsRepo.Set(ctx, "ai_claude_client_id", *req.ClaudeClientID, false); err != nil {
			log.Error().Err(err).Msg("failed to set claude client id")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.ClaudeClientSecret != nil {
		if err := h.settingsRepo.Set(ctx, "ai_claude_client_secret", *req.ClaudeClientSecret, true); err != nil {
			log.Error().Err(err).Msg("failed to set claude client secret")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.AIEnabled != nil {
		val := "false"
		if *req.AIEnabled {
			val = "true"
		}
		if err := h.settingsRepo.Set(ctx, "ai_enabled", val, false); err != nil {
			log.Error().Err(err).Msg("failed to set ai enabled")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	if req.AIModel != nil {
		if err := h.settingsRepo.Set(ctx, "ai_model", *req.AIModel, false); err != nil {
			log.Error().Err(err).Msg("failed to set ai model")
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}

	log.Info().Msg("AI settings updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "settings updated"})
}

// TestAIConnection tests the Anthropic API connection by sending a minimal request.
// Supports both API key and OAuth token authentication.
func (h *SettingsHandler) TestAIConnection(w http.ResponseWriter, r *http.Request) {
	log := logger.With("settings")
	ctx := r.Context()

	var req struct {
		APIKey       string `json:"apiKey"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    string `json:"expiresAt"`
		AuthMode     string `json:"authMode"`
		Model        string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Info().
		Str("authMode", req.AuthMode).
		Bool("hasApiKey", req.APIKey != "").
		Bool("hasAccessToken", req.AccessToken != "").
		Bool("hasRefreshToken", req.RefreshToken != "").
		Str("model", req.Model).
		Msg("test connection request received")

	da := agent.NewDeployAgent()

	// Resolve the token to use based on auth mode
	var token string
	authMode := req.AuthMode

	if authMode == "" {
		stored, _ := h.settingsRepo.Get(ctx, "ai_auth_mode")
		authMode = stored
	}

	if authMode == "oauth" {
		token = req.AccessToken
		if token == "" {
			stored, _ := h.settingsRepo.Get(ctx, "ai_oauth_access_token")
			token = stored
		}
		// Try refresh if we have a refresh token and no valid access token
		if token == "" {
			refreshTok := req.RefreshToken
			if refreshTok == "" {
				refreshTok, _ = h.settingsRepo.Get(ctx, "ai_oauth_refresh_token")
			}
			if refreshTok == "" {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   "no OAuth token provided and none stored",
				})
				return
			}
			result, err := da.RefreshOAuthToken(ctx, refreshTok)
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   "OAuth refresh failed: " + err.Error(),
				})
				return
			}
			token = result.AccessToken
		}
	} else {
		token = req.APIKey
		if token == "" {
			stored, _ := h.settingsRepo.Get(ctx, "ai_anthropic_api_key")
			token = stored
		}
	}

	if token == "" {
		log.Warn().Str("resolvedAuthMode", authMode).Msg("no credentials found for test connection")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "no credentials provided and none stored",
		})
		return
	}

	model := req.Model
	if model == "" {
		stored, _ := h.settingsRepo.Get(ctx, "ai_model")
		if stored != "" {
			model = stored
		}
	}

	usedModel, err := da.TestConnection(ctx, token, model)
	if err != nil {
		log.Warn().Err(err).Msg("AI connection test failed")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Info().Str("model", usedModel).Msg("AI connection test successful")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"model":   usedModel,
	})
}
