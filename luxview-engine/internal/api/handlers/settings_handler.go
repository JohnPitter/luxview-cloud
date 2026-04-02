package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/luxview/engine/internal/agent"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
	auditSvc     *service.AuditService
}

func NewSettingsHandler(settingsRepo *repository.SettingsRepo, auditSvc *service.AuditService) *SettingsHandler {
	return &SettingsHandler{settingsRepo: settingsRepo, auditSvc: auditSvc}
}

// aiSettingsResponse is the JSON shape returned by GetAISettings.
// JSON tags use snake_case; the frontend axios interceptor converts to camelCase automatically.
type aiSettingsResponse struct {
	APIKey    string `json:"api_key"`
	AIEnabled bool   `json:"ai_enabled"`
	AIModel   string `json:"ai_model"`
}

// updateAISettingsRequest accepts partial updates (pointer fields).
// JSON tags use snake_case because the frontend axios interceptor converts camelCase → snake_case.
type updateAISettingsRequest struct {
	APIKey    *string `json:"api_key"`
	AIEnabled *bool   `json:"ai_enabled"`
	AIModel   *string `json:"ai_model"`
}

// maskSecret masks a string showing only the first 4 and last 4 characters.
// If the string is too short (<=8), it returns a fixed mask.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••••••"
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
		APIKey:    maskSecret(settings["api_key"]),
		AIEnabled: settings["enabled"] == "true",
		AIModel:   settings["model"],
	}

	if resp.AIModel == "" {
		resp.AIModel = "anthropic/claude-sonnet-4"
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

	if req.APIKey != nil {
		if err := h.settingsRepo.Set(ctx, "ai_api_key", *req.APIKey, true); err != nil {
			log.Error().Err(err).Msg("failed to set api key")
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

	user := middleware.GetUser(ctx)
	auditNewValues := map[string]interface{}{}
	if req.AIEnabled != nil {
		auditNewValues["aiEnabled"] = *req.AIEnabled
	}
	if req.AIModel != nil {
		auditNewValues["aiModel"] = *req.AIModel
	}
	if req.APIKey != nil {
		auditNewValues["apiKeyChanged"] = true
	}
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "setting",
		ResourceID:   "ai",
		ResourceName: "ai",
		NewValues:    auditNewValues,
		IPAddress:    clientIP(r),
	})

	log.Info().Msg("AI settings updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "settings updated"})
}

// TestAIConnection tests the OpenRouter API connection by sending a minimal request.
func (h *SettingsHandler) TestAIConnection(w http.ResponseWriter, r *http.Request) {
	log := logger.With("settings")
	ctx := r.Context()

	var req struct {
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	da := agent.NewDeployAgent()

	token := req.APIKey
	if token == "" {
		stored, _ := h.settingsRepo.Get(ctx, "ai_api_key")
		token = stored
	}

	if token == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "no API key provided and none stored",
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

// GetAuthSettings returns platform auth configuration.
func (h *SettingsHandler) GetAuthSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	val, _ := h.settingsRepo.Get(ctx, "platform_require_auth")
	requireAuth := val != "false" // default true
	writeJSON(w, http.StatusOK, map[string]bool{"require_auth": requireAuth})
}

// UpdateAuthSettings updates platform auth configuration.
func (h *SettingsHandler) UpdateAuthSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		RequireAuth bool `json:"require_auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	val := "true"
	if !req.RequireAuth {
		val = "false"
	}
	if err := h.settingsRepo.Set(ctx, "platform_require_auth", val, false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update auth settings")
		return
	}
	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "setting",
		ResourceID:   "auth",
		ResourceName: "auth",
		NewValues:    map[string]interface{}{"require_auth": req.RequireAuth},
		IPAddress:    clientIP(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "auth settings updated"})
}

// GetTimezone returns the platform timezone setting.
func (h *SettingsHandler) GetTimezone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tz, _ := h.settingsRepo.Get(ctx, "platform_timezone")
	if tz == "" {
		tz = "UTC"
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// UpdateTimezone sets the platform timezone.
func (h *SettingsHandler) UpdateTimezone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Timezone == "" {
		writeError(w, http.StatusBadRequest, "timezone is required")
		return
	}

	if err := h.settingsRepo.Set(ctx, "platform_timezone", req.Timezone, false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update timezone")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:      user.ID,
		ActorUsername: user.Username,
		Action:       "update",
		ResourceType: "setting",
		ResourceID:   "timezone",
		ResourceName: "timezone",
		NewValues:    map[string]interface{}{"timezone": req.Timezone},
		IPAddress:    clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "timezone updated"})
}
