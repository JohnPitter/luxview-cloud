package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
)

type AlertHandler struct {
	alertRepo *repository.AlertRepo
	appRepo   *repository.AppRepo
}

func NewAlertHandler(alertRepo *repository.AlertRepo, appRepo *repository.AppRepo) *AlertHandler {
	return &AlertHandler{
		alertRepo: alertRepo,
		appRepo:   appRepo,
	}
}

// Create creates a new alert for an app.
func (h *AlertHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req model.CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate metric
	validMetrics := map[string]bool{"cpu_percent": true, "memory_bytes": true, "network_rx": true, "network_tx": true}
	if !validMetrics[req.Metric] {
		writeError(w, http.StatusBadRequest, "invalid metric")
		return
	}

	// Validate condition
	validConditions := map[string]bool{"gt": true, "gte": true, "lt": true, "lte": true, "eq": true}
	if !validConditions[req.Condition] {
		writeError(w, http.StatusBadRequest, "invalid condition")
		return
	}

	alert := &model.Alert{
		AppID:         appID,
		Metric:        req.Metric,
		Condition:     req.Condition,
		Threshold:     req.Threshold,
		Channel:       req.Channel,
		ChannelConfig: req.ChannelConfig,
		Enabled:       true,
	}

	if err := h.alertRepo.Create(ctx, alert); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create alert")
		return
	}

	writeJSON(w, http.StatusCreated, alert)
}

// List lists alerts for an app.
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	alerts, err := h.alertRepo.ListByAppID(ctx, appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list alerts")
		return
	}

	writeJSON(w, http.StatusOK, alerts)
}

// Update updates an alert.
func (h *AlertHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	alertID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert ID")
		return
	}

	alert, err := h.alertRepo.FindByID(ctx, alertID)
	if err != nil || alert == nil {
		writeError(w, http.StatusNotFound, "alert not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, alert.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req model.UpdateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Metric != nil {
		alert.Metric = *req.Metric
	}
	if req.Condition != nil {
		alert.Condition = *req.Condition
	}
	if req.Threshold != nil {
		alert.Threshold = *req.Threshold
	}
	if req.Channel != nil {
		alert.Channel = *req.Channel
	}
	if req.ChannelConfig != nil {
		alert.ChannelConfig = req.ChannelConfig
	}
	if req.Enabled != nil {
		alert.Enabled = *req.Enabled
	}

	if err := h.alertRepo.Update(ctx, alert); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update alert")
		return
	}

	writeJSON(w, http.StatusOK, alert)
}

// Delete deletes an alert.
func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	alertID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert ID")
		return
	}

	alert, err := h.alertRepo.FindByID(ctx, alertID)
	if err != nil || alert == nil {
		writeError(w, http.StatusNotFound, "alert not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, alert.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.alertRepo.Delete(ctx, alertID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete alert")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "alert deleted"})
}
