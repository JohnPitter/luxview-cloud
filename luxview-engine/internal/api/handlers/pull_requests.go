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

type PullRequestHandler struct {
	repositoryRepo *repository.RepositoryRepo
	prSvc          *service.PullRequestService
	auditSvc       *service.AuditService
}

func NewPullRequestHandler(repositoryRepo *repository.RepositoryRepo, prSvc *service.PullRequestService, auditSvc *service.AuditService) *PullRequestHandler {
	return &PullRequestHandler{repositoryRepo: repositoryRepo, prSvc: prSvc, auditSvc: auditSvc}
}

// authorizePR resolves the repository and verifies ownership.
func (h *PullRequestHandler) authorizeRepository(w http.ResponseWriter, r *http.Request) (*model.Repository, bool) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	repoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repository ID")
		return nil, false
	}
	repo, err := h.repositoryRepo.FindByID(ctx, repoID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return nil, false
	}
	if repo.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return repo, true
}

func (h *PullRequestHandler) prNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "number"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "invalid pull request number")
		return 0, false
	}
	return n, true
}

// List GET /repositories/{id}/pulls?status=open
func (h *PullRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	prs, total, err := h.prSvc.List(r.Context(), repo.ID, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pull requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pull_requests": prs, "total": total})
}

// Create POST /repositories/{id}/pulls
func (h *PullRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		HeadBranch  string `json:"head_branch"`
		BaseBranch  string `json:"base_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pr, err := h.prSvc.Create(r.Context(), service.CreatePRRequest{
		RepositoryID: repo.ID,
		AuthorID:     user.ID,
		Title:        req.Title,
		Description:  req.Description,
		HeadBranch:   req.HeadBranch,
		BaseBranch:   req.BaseBranch,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Log(r.Context(), service.AuditEntry{
			ActorID: user.ID, ActorUsername: user.Username,
			Action: "create", ResourceType: "pull_request",
			ResourceID: pr.ID.String(), ResourceName: pr.Title,
			IPAddress: clientIP(r),
		})
	}

	writeJSON(w, http.StatusCreated, pr)
}

// Get GET /repositories/{id}/pulls/{number}
func (h *PullRequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}

	pr, err := h.prSvc.Get(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

// Commits GET /repositories/{id}/pulls/{number}/commits
func (h *PullRequestHandler) Commits(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}

	commits, err := h.prSvc.Commits(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

// Diff GET /repositories/{id}/pulls/{number}/diff
func (h *PullRequestHandler) Diff(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}

	diffs, err := h.prSvc.Diff(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": diffs})
}

// Merge POST /repositories/{id}/pulls/{number}/merge
func (h *PullRequestHandler) Merge(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	pr, err := h.prSvc.Merge(r.Context(), repo.ID, number, user.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Log(r.Context(), service.AuditEntry{
			ActorID: user.ID, ActorUsername: user.Username,
			Action: "merge", ResourceType: "pull_request",
			ResourceID: pr.ID.String(), ResourceName: pr.Title,
			IPAddress: clientIP(r),
		})
	}

	writeJSON(w, http.StatusOK, pr)
}

// Close POST /repositories/{id}/pulls/{number}/close
func (h *PullRequestHandler) Close(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	pr, err := h.prSvc.Close(r.Context(), repo.ID, number, user.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

// ListComments GET /repositories/{id}/pulls/{number}/comments
func (h *PullRequestHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}

	pr, err := h.prSvc.Get(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	comments, err := h.prSvc.ListComments(r.Context(), pr.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

// AddComment POST /repositories/{id}/pulls/{number}/comments
func (h *PullRequestHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.prNumber(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	pr, err := h.prSvc.Get(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	comment, err := h.prSvc.AddComment(r.Context(), pr.ID, user.ID, req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// DeleteComment DELETE /repositories/{id}/pulls/{number}/comments/{commentId}
func (h *PullRequestHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	_, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())
	commentID, err := uuid.Parse(chi.URLParam(r, "commentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	if err := h.prSvc.DeleteComment(r.Context(), commentID, user.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
