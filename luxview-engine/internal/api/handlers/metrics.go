package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/repository"
)

type MetricHandler struct {
	metricRepo *repository.MetricRepo
	appRepo    *repository.AppRepo
}

func NewMetricHandler(metricRepo *repository.MetricRepo, appRepo *repository.AppRepo) *MetricHandler {
	return &MetricHandler{
		metricRepo: metricRepo,
		appRepo:    appRepo,
	}
}

// Get returns aggregated metrics for an app.
func (h *MetricHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	// Parse time range
	from := parseTime(r.URL.Query().Get("from"), time.Now().Add(-1*time.Hour))
	to := parseTime(r.URL.Query().Get("to"), time.Now())

	interval, _ := strconv.Atoi(r.URL.Query().Get("interval"))
	if interval <= 0 {
		interval = 60 // default 1 minute buckets
	}

	metrics, err := h.metricRepo.GetAggregated(ctx, appID, from, to, interval)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get metrics")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"metrics":  metrics,
		"from":     from,
		"to":       to,
		"interval": interval,
	})
}

func parseTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try unix timestamp
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
		return fallback
	}
	return t
}
