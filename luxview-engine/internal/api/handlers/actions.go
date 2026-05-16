package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

type ActionHandler struct {
	actionRepo *repository.ActionRepo
	appRepo    *repository.AppRepo
	actionSvc  *service.ActionService
	auditSvc   *service.AuditService
}

type upsertActionSecretRequest struct {
	Value string `json:"value"`
}

func NewActionHandler(actionRepo *repository.ActionRepo, appRepo *repository.AppRepo, actionSvc *service.ActionService, auditSvc *service.AuditService) *ActionHandler {
	return &ActionHandler{actionRepo: actionRepo, appRepo: appRepo, actionSvc: actionSvc, auditSvc: auditSvc}
}

func (h *ActionHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	app, ok := h.authorizeApp(w, r)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	runs, total, err := h.actionRepo.ListRunsByAppID(ctx, app.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list action runs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"runs": runs, "total": total})
}

func (h *ActionHandler) TriggerRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	app, ok := h.authorizeApp(w, r)
	if !ok {
		return
	}

	var req service.TriggerActionRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	run, err := h.actionSvc.TriggerRun(ctx, app.ID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:       user.ID,
		ActorUsername: user.Username,
		Action:        "create",
		ResourceType:  "action_run",
		ResourceID:    run.ID.String(),
		ResourceName:  app.Subdomain,
		NewValues:     map[string]string{"workflow": run.Workflow, "workflow_path": run.WorkflowPath},
		IPAddress:     clientIP(r),
	})

	writeJSON(w, http.StatusAccepted, run)
}

func (h *ActionHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action run ID")
		return
	}

	detail, err := h.actionRepo.FindRunDetail(ctx, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get action run")
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "action run not found")
		return
	}

	app, err := h.appRepo.FindByID(ctx, detail.Run.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *ActionHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid action run ID")
		return
	}

	detail, err := h.actionRepo.FindRunDetail(ctx, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get action run")
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "action run not found")
		return
	}
	app, err := h.appRepo.FindByID(ctx, detail.Run.AppID)
	if err != nil || app == nil || app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	artifacts, err := h.actionRepo.ListArtifactsByRunID(ctx, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list action artifacts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"artifacts": artifacts})
}

func (h *ActionHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	app, ok := h.authorizeApp(w, r)
	if !ok {
		return
	}

	secrets, err := h.actionRepo.ListSecrets(ctx, app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list action secrets")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"secrets": secrets})
}

func (h *ActionHandler) UpsertSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	app, ok := h.authorizeApp(w, r)
	if !ok {
		return
	}

	key := chi.URLParam(r, "key")
	if !service.IsValidActionSecretKey(key) {
		writeError(w, http.StatusBadRequest, "invalid secret key")
		return
	}

	var req upsertActionSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Value == "" {
		writeError(w, http.StatusBadRequest, "secret value is required")
		return
	}

	secret, err := h.actionRepo.UpsertSecret(ctx, app.ID, key, req.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save action secret")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:       user.ID,
		ActorUsername: user.Username,
		Action:        "update",
		ResourceType:  "action_secret",
		ResourceID:    secret.ID.String(),
		ResourceName:  key,
		NewValues:     map[string]string{"key": key},
		IPAddress:     clientIP(r),
	})

	writeJSON(w, http.StatusOK, secret)
}

func (h *ActionHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	app, ok := h.authorizeApp(w, r)
	if !ok {
		return
	}

	key := chi.URLParam(r, "key")
	if !service.IsValidActionSecretKey(key) {
		writeError(w, http.StatusBadRequest, "invalid secret key")
		return
	}

	if err := h.actionRepo.DeleteSecret(ctx, app.ID, key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete action secret")
		return
	}

	user := middleware.GetUser(ctx)
	h.auditSvc.Log(ctx, service.AuditEntry{
		ActorID:       user.ID,
		ActorUsername: user.Username,
		Action:        "delete",
		ResourceType:  "action_secret",
		ResourceName:  key,
		OldValues:     map[string]string{"key": key},
		IPAddress:     clientIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "secret deleted"})
}

func (h *ActionHandler) authorizeApp(w http.ResponseWriter, r *http.Request) (*model.App, bool) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app ID")
		return nil, false
	}
	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return nil, false
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return app, true
}
