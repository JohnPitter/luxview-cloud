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

type RepositoryHandler struct {
	repositoryRepo *repository.RepositoryRepo
	repositorySvc  *service.RepositoryService
	auditSvc       *service.AuditService
}

type createRepositoryRequest struct {
	Name          string                     `json:"name"`
	Slug          string                     `json:"slug"`
	DefaultBranch string                     `json:"default_branch"`
	Visibility    model.RepositoryVisibility `json:"visibility"`
}

func NewRepositoryHandler(repositoryRepo *repository.RepositoryRepo, repositorySvc *service.RepositoryService, auditSvc *service.AuditService) *RepositoryHandler {
	return &RepositoryHandler{repositoryRepo: repositoryRepo, repositorySvc: repositorySvc, auditSvc: auditSvc}
}

func (h *RepositoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	repo, err := h.repositorySvc.Create(ctx, service.CreateRepositoryRequest{
		UserID:        user.ID,
		Name:          req.Name,
		Slug:          req.Slug,
		DefaultBranch: req.DefaultBranch,
		Visibility:    req.Visibility,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Log(ctx, service.AuditEntry{
			ActorID:       user.ID,
			ActorUsername: user.Username,
			Action:        "create",
			ResourceType:  "repository",
			ResourceID:    repo.ID.String(),
			ResourceName:  repo.Slug,
			NewValues: map[string]string{
				"name": repo.Name,
				"slug": repo.Slug,
			},
			IPAddress: clientIP(r),
		})
	}

	writeJSON(w, http.StatusCreated, repo)
}

func (h *RepositoryHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	repos, total, err := h.repositoryRepo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"repositories": repos,
		"total":        total,
	})
}

func (h *RepositoryHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	repositoryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repository ID")
		return
	}
	repo, err := h.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}
	if repo.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	branches, err := h.repositorySvc.ListBranches(ctx, repositoryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, branches)
}
