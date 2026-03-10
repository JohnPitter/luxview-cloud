package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type DeploymentHandler struct {
	deployRepo *repository.DeploymentRepo
	appRepo    *repository.AppRepo
	buildQueue chan<- service.DeployRequest
	auditSvc   *service.AuditService
}

func NewDeploymentHandler(deployRepo *repository.DeploymentRepo, appRepo *repository.AppRepo, buildQueue chan<- service.DeployRequest, auditSvc *service.AuditService) *DeploymentHandler {
	return &DeploymentHandler{
		deployRepo: deployRepo,
		appRepo:    appRepo,
		buildQueue: buildQueue,
		auditSvc:   auditSvc,
	}
}

// List lists deployments for an app.
func (h *DeploymentHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	deployments, total, err := h.deployRepo.ListByAppID(ctx, appID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deployments": deployments,
		"total":       total,
	})
}

// GetLogs returns the build log for a deployment.
func (h *DeploymentHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	deployID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment ID")
		return
	}

	deployment, err := h.deployRepo.FindByID(ctx, deployID)
	if err != nil || deployment == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, deployment.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        deployment.ID,
		"status":    deployment.Status,
		"build_log": deployment.BuildLog,
	})
}

// Rollback triggers a rollback to a specific deployment.
func (h *DeploymentHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	log := logger.With("deployments")
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	deployID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment ID")
		return
	}

	deployment, err := h.deployRepo.FindByID(ctx, deployID)
	if err != nil || deployment == nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, deployment.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if deployment.ImageTag == "" {
		writeError(w, http.StatusBadRequest, "deployment has no image to rollback to")
		return
	}

	req := service.DeployRequest{
		AppID:     app.ID,
		UserID:    userID,
		CommitSHA: deployment.CommitSHA,
		CommitMsg: "rollback to " + deployment.CommitSHA,
	}

	select {
	case h.buildQueue <- req:
		user := middleware.GetUser(ctx)
		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:      user.ID,
			ActorUsername: user.Username,
			Action:       "deploy",
			ResourceType: "deployment",
			ResourceID:   deployID.String(),
			ResourceName: app.Subdomain,
			NewValues:    map[string]string{"rollbackToDeployId": deployID.String()},
			IPAddress:    clientIP(r),
		})
		log.Info().Str("app", app.Subdomain).Str("deploy", deployID.String()).Msg("rollback queued")
		writeJSON(w, http.StatusAccepted, map[string]string{"message": "rollback queued"})
	default:
		writeError(w, http.StatusServiceUnavailable, "build queue full")
	}
}
