package handlers

import (
	"io"
	"net/http"

	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type WebhookHandler struct {
	webhookSvc *service.WebhookService
	secret     string
}

func NewWebhookHandler(webhookSvc *service.WebhookService, secret string) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc: webhookSvc,
		secret:     secret,
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

	// Verify signature if secret is configured
	if h.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !service.VerifySignature(body, signature, h.secret) {
			log.Warn().Msg("invalid webhook signature")
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "push" {
		writeJSON(w, http.StatusOK, map[string]string{"message": "event ignored"})
		return
	}

	if err := h.webhookSvc.ProcessPush(r.Context(), body); err != nil {
		log.Error().Err(err).Msg("failed to process push event")
		writeError(w, http.StatusInternalServerError, "failed to process webhook")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}
