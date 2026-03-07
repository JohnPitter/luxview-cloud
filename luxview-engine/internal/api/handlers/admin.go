package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type AdminHandler struct {
	userRepo   *repository.UserRepo
	appRepo    *repository.AppRepo
	deployRepo *repository.DeploymentRepo
	container  *service.ContainerManager
}

func NewAdminHandler(
	userRepo *repository.UserRepo,
	appRepo *repository.AppRepo,
	deployRepo *repository.DeploymentRepo,
	container *service.ContainerManager,
) *AdminHandler {
	return &AdminHandler{
		userRepo:   userRepo,
		appRepo:    appRepo,
		deployRepo: deployRepo,
		container:  container,
	}
}

// ListUsers lists all users (admin only).
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	users, total, err := h.userRepo.ListAll(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Convert to response format (no sensitive fields)
	var responses []model.UserResponse
	for _, u := range users {
		responses = append(responses, u.ToResponse())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": responses,
		"total": total,
	})
}

// Stats returns global platform stats.
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, userTotal, _ := h.userRepo.ListAll(ctx, 1, 0)
	appTotal, _ := h.appRepo.CountAll(ctx)
	runningApps, _ := h.appRepo.CountByStatus(ctx, model.AppStatusRunning)
	deployTotal, _ := h.deployRepo.CountAll(ctx)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_users":       userTotal,
		"total_apps":        appTotal,
		"running_apps":      runningApps,
		"total_deployments": deployTotal,
	})
}

// ForceDeleteApp force-deletes any app (admin only).
func (h *AdminHandler) ForceDeleteApp(w http.ResponseWriter, r *http.Request) {
	log := logger.With("admin")
	ctx := r.Context()

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

	if app.ContainerID != "" {
		_ = h.container.Stop(ctx, app.ContainerID)
		_ = h.container.Remove(ctx, app.ContainerID)
	}

	if err := h.appRepo.Delete(ctx, appID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}

	log.Info().Str("app", app.Subdomain).Msg("admin force-deleted app")
	writeJSON(w, http.StatusOK, map[string]string{"message": "app force deleted"})
}

// UpdateUserRole changes a user's role (admin only).
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var body struct {
		Role model.UserRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Role != model.RoleUser && body.Role != model.RoleAdmin {
		writeError(w, http.StatusBadRequest, "role must be 'user' or 'admin'")
		return
	}

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := h.userRepo.UpdateRole(ctx, userID, body.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	log := logger.With("admin")
	log.Info().Str("user", user.Username).Str("role", string(body.Role)).Msg("user role updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "role updated"})
}

// UpdateAppLimits changes an app's resource limits (admin only).
func (h *AdminHandler) UpdateAppLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return
	}

	var body struct {
		ResourceLimits model.ResourceLimits `json:"resource_limits"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}

	if err := h.appRepo.UpdateResourceLimits(ctx, appID, body.ResourceLimits); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update limits")
		return
	}

	log := logger.With("admin")
	log.Info().Str("app", app.Subdomain).Msg("app resource limits updated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "limits updated"})
}

// ListAllApps returns all apps across all users (admin only).
func (h *AdminHandler) ListAllApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	apps, total, err := h.appRepo.ListAll(ctx, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"apps":  apps,
		"total": total,
	})
}
