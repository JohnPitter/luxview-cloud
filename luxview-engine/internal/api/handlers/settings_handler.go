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
	AnthropicAPIKey   string `json:"anthropicApiKey"`
	ClaudeClientID    string `json:"claudeClientId"`
	ClaudeClientSecret string `json:"claudeClientSecret"`
	AIEnabled         bool   `json:"aiEnabled"`
	AIModel           string `json:"aiModel"`
}

// updateAISettingsRequest accepts partial updates (pointer fields).
type updateAISettingsRequest struct {
	AnthropicAPIKey    *string `json:"anthropicApiKey"`
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

	resp := aiSettingsResponse{
		AnthropicAPIKey:    maskSecret(settings["anthropic_api_key"]),
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

	if req.AnthropicAPIKey != nil {
		if err := h.settingsRepo.Set(ctx, "ai_anthropic_api_key", *req.AnthropicAPIKey, true); err != nil {
			log.Error().Err(err).Msg("failed to set anthropic api key")
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

// TestAIConnection tests the Anthropic API key by sending a minimal request.
func (h *SettingsHandler) TestAIConnection(w http.ResponseWriter, r *http.Request) {
	log := logger.With("settings")
	ctx := r.Context()

	// Get the API key — either from the request body (if user is testing a new key)
	// or from the stored settings
	var req struct {
		APIKey string `json:"apiKey"`
		Model  string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	apiKey := req.APIKey
	if apiKey == "" {
		// Fall back to stored key
		stored, err := h.settingsRepo.Get(ctx, "ai_anthropic_api_key")
		if err != nil || stored == "" {
			writeError(w, http.StatusBadRequest, "no API key provided and none stored")
			return
		}
		apiKey = stored
	}

	model := req.Model
	if model == "" {
		stored, _ := h.settingsRepo.Get(ctx, "ai_model")
		if stored != "" {
			model = stored
		}
	}

	// Test connection
	da := agent.NewDeployAgent()
	usedModel, err := da.TestConnection(ctx, apiKey, model)
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
