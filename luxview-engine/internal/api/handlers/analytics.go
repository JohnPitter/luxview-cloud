package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
)

type AnalyticsHandler struct {
	pageviewRepo *repository.PageviewRepo
	appRepo      *repository.AppRepo
}

func NewAnalyticsHandler(pageviewRepo *repository.PageviewRepo, appRepo *repository.AppRepo) *AnalyticsHandler {
	return &AnalyticsHandler{
		pageviewRepo: pageviewRepo,
		appRepo:      appRepo,
	}
}

// parseAnalyticsParams extracts and validates common analytics query params.
// Returns appID (nil for platform), start, end, and whether the request is valid.
func (h *AnalyticsHandler) parseAnalyticsParams(w http.ResponseWriter, r *http.Request) (*uuid.UUID, time.Time, time.Time, bool) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, time.Time{}, time.Time{}, false
	}

	appIDStr := r.URL.Query().Get("app_id")
	var appID *uuid.UUID

	if appIDStr != "" {
		id, err := uuid.Parse(appIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid app_id")
			return nil, time.Time{}, time.Time{}, false
		}

		// Verify ownership
		app, err := h.appRepo.FindByID(r.Context(), id)
		if err != nil || app == nil {
			writeError(w, http.StatusNotFound, "app not found")
			return nil, time.Time{}, time.Time{}, false
		}
		if app.UserID != user.ID && user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden")
			return nil, time.Time{}, time.Time{}, false
		}
		appID = &id
	} else {
		// Platform analytics — admin only
		if user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "platform analytics require admin role")
			return nil, time.Time{}, time.Time{}, false
		}
	}

	start := parseTime(r.URL.Query().Get("start"), time.Now().Add(-24*time.Hour))
	end := parseTime(r.URL.Query().Get("end"), time.Now())

	return appID, start, end, true
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity != "day" {
		granularity = "hour"
	}

	result, err := h.pageviewRepo.Overview(r.Context(), appID, start, end, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get analytics overview")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, result)
}

func (h *AnalyticsHandler) Pages(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 20)

	items, err := h.pageviewRepo.TopPages(r.Context(), appID, start, end, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get top pages")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"pages": items})
}

func (h *AnalyticsHandler) Geo(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 20)

	items, err := h.pageviewRepo.TopGeo(r.Context(), appID, start, end, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get geo data")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"countries": items})
}

func (h *AnalyticsHandler) Browsers(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	items, err := h.pageviewRepo.Breakdown(r.Context(), appID, start, end, "browser")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get browser data")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"browsers": items})
}

func (h *AnalyticsHandler) OS(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	items, err := h.pageviewRepo.Breakdown(r.Context(), appID, start, end, "os")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get OS data")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"os": items})
}

func (h *AnalyticsHandler) Devices(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	items, err := h.pageviewRepo.Breakdown(r.Context(), appID, start, end, "device_type")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get device data")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"devices": items})
}

func (h *AnalyticsHandler) Referers(w http.ResponseWriter, r *http.Request) {
	appID, start, end, ok := h.parseAnalyticsParams(w, r)
	if !ok {
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 20)

	items, err := h.pageviewRepo.TopReferers(r.Context(), appID, start, end, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get referer data")
		return
	}

	w.Header().Set("Cache-Control", "max-age=60")
	writeJSON(w, http.StatusOK, map[string]interface{}{"referers": items})
}

func (h *AnalyticsHandler) Live(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	appIDStr := r.URL.Query().Get("app_id")
	var appID *uuid.UUID

	if appIDStr != "" {
		id, err := uuid.Parse(appIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid app_id")
			return
		}
		app, err := h.appRepo.FindByID(r.Context(), id)
		if err != nil || app == nil {
			writeError(w, http.StatusNotFound, "app not found")
			return
		}
		if app.UserID != user.ID && user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		appID = &id
	} else {
		if user.Role != model.RoleAdmin {
			writeError(w, http.StatusForbidden, "platform analytics require admin role")
			return
		}
	}

	count, err := h.pageviewRepo.LiveVisitors(r.Context(), appID, 5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get live visitors")
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, http.StatusOK, map[string]interface{}{"visitors": count})
}

func parseLimit(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 || v > 100 {
		return fallback
	}
	return v
}
