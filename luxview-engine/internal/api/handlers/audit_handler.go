package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/repository"
)

type AuditHandler struct {
	auditRepo *repository.AuditLogRepo
}

func NewAuditHandler(auditRepo *repository.AuditLogRepo) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

func (h *AuditHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	filter := repository.AuditFilter{
		Action:       r.URL.Query().Get("action"),
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		Search:       r.URL.Query().Get("search"),
	}

	if actorStr := r.URL.Query().Get("actor_id"); actorStr != "" {
		if id, err := uuid.Parse(actorStr); err == nil {
			filter.ActorID = &id
		}
	}
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = &t
		}
	}

	logs, err := h.auditRepo.List(ctx, filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}
	total, _ := h.auditRepo.Count(ctx, filter)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": total,
	})
}

func (h *AuditHandler) AuditStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	since := time.Now().Add(-24 * time.Hour)

	total, _ := h.auditRepo.CountSince(ctx, since)
	byAction, _ := h.auditRepo.StatsByAction(ctx, since)
	byResource, _ := h.auditRepo.StatsByResource(ctx, since)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_24h":   total,
		"by_action":   byAction,
		"by_resource": byResource,
	})
}
