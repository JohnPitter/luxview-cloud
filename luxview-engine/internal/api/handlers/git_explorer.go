package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

type GitExplorerHandler struct {
	repositoryRepo *repository.RepositoryRepo
	repositorySvc  *service.RepositoryService
}

func NewGitExplorerHandler(repositoryRepo *repository.RepositoryRepo, repositorySvc *service.RepositoryService) *GitExplorerHandler {
	return &GitExplorerHandler{repositoryRepo: repositoryRepo, repositorySvc: repositorySvc}
}

func (h *GitExplorerHandler) resolveRepo(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	repositoryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repository ID")
		return uuid.Nil, false
	}
	repo, err := h.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return uuid.Nil, false
	}
	if repo.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return uuid.Nil, false
	}
	return repositoryID, true
}

func (h *GitExplorerHandler) Tree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	path := r.URL.Query().Get("path")
	entries, err := h.repositorySvc.ListTree(ctx, id, ref, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries, "path": path, "ref": ref})
}

func (h *GitExplorerHandler) Blob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	content, err := h.repositorySvc.GetBlob(ctx, id, ref, path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"content": string(content), "path": path, "ref": ref})
}

func (h *GitExplorerHandler) Commits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	commits, err := h.repositorySvc.ListCommits(ctx, id, ref, limit, offset)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"commits": commits})
}

func (h *GitExplorerHandler) Commit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	sha := chi.URLParam(r, "sha")
	commit, diffs, err := h.repositorySvc.GetCommit(ctx, id, sha)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"commit": commit, "files": diffs})
}

func (h *GitExplorerHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	tags, err := h.repositorySvc.ListTags(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})
}

type createTagRequest struct {
	Name    string `json:"name"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

func (h *GitExplorerHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	var req createTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.repositorySvc.CreateTag(ctx, id, req.Name, req.Ref, req.Message); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (h *GitExplorerHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	if err := h.repositorySvc.DeleteTag(ctx, id, name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createBranchRequest struct {
	Name string `json:"name"`
	From string `json:"from"`
}

func (h *GitExplorerHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	var req createBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.repositorySvc.CreateBranch(ctx, id, req.Name, req.From); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (h *GitExplorerHandler) DeleteBranch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	if err := h.repositorySvc.DeleteBranch(ctx, id, name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
