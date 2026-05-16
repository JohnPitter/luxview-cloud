package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type WebhookHandler struct {
	webhookSvc   *service.WebhookService
	githubAppSvc *service.GitHubAppService
	secret       string
	appSecret    string // GitHub App webhook secret (takes priority when set)
}

func NewWebhookHandler(webhookSvc *service.WebhookService, secret, appSecret string, githubAppSvc *service.GitHubAppService) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc:   webhookSvc,
		githubAppSvc: githubAppSvc,
		secret:       secret,
		appSecret:    appSecret,
	}
}

// GitHubWebhook processes incoming GitHub webhook events.
func (h *WebhookHandler) GitHubWebhook(w http.ResponseWriter, r *http.Request) {
	log := logger.With("webhook")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	log.Debug().Str("event_type", eventType).Int("body_size", len(body)).Msg("incoming webhook event")

	// Prefer GitHub App webhook secret, fall back to InternalToken.
	activeSecret := h.appSecret
	if activeSecret == "" {
		activeSecret = h.secret
	}

	if activeSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !service.VerifySignature(body, signature, activeSecret) {
			log.Warn().Msg("invalid webhook signature")
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
		log.Debug().Msg("webhook signature verified")
	} else {
		log.Debug().Msg("webhook signature verification skipped (no secret configured)")
	}

	switch eventType {
	case "push":
		log.Debug().Msg("calling ProcessPush")
		if err := h.webhookSvc.ProcessPush(r.Context(), body); err != nil {
			log.Error().Err(err).Msg("failed to process push event")
			writeError(w, http.StatusInternalServerError, "failed to process webhook")
			return
		}

	case "installation":
		if h.githubAppSvc != nil {
			h.handleInstallationEvent(r, body)
		}

	default:
		log.Debug().Str("event_type", eventType).Msg("ignoring event")
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

func (h *WebhookHandler) handleInstallationEvent(r *http.Request, body []byte) {
	log := logger.With("webhook")
	var event struct {
		Action       string `json:"action"`
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
		Sender struct {
			ID int64 `json:"id"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		log.Warn().Err(err).Msg("failed to parse installation event")
		return
	}
	ctx := r.Context()
	switch event.Action {
	case "created":
		if err := h.githubAppSvc.HandleInstallation(ctx, event.Installation.ID, event.Sender.ID); err != nil {
			log.Error().Err(err).Msg("failed to handle app installation")
		}
	case "deleted":
		if err := h.githubAppSvc.HandleUninstallation(ctx, event.Installation.ID); err != nil {
			log.Error().Err(err).Msg("failed to handle app uninstallation")
		}
	}
}
