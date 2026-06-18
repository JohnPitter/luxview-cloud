package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

type BranchProtectionHandler struct {
	repositoryRepo *repository.RepositoryRepo
	bpRepo         *repository.BranchProtectionRepo
	auditSvc       *service.AuditService
}

func NewBranchProtectionHandler(repositoryRepo *repository.RepositoryRepo, bpRepo *repository.BranchProtectionRepo, auditSvc *service.AuditService) *BranchProtectionHandler {
	return &BranchProtectionHandler{repositoryRepo: repositoryRepo, bpRepo: bpRepo, auditSvc: auditSvc}
}

func (h *BranchProtectionHandler) authorizeRepository(w http.ResponseWriter, r *http.Request) (*model.Repository, bool) {
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

// List GET /repositories/{id}/branch-protection
func (h *BranchProtectionHandler) List(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	rules, err := h.bpRepo.ListByRepository(r.Context(), repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// Upsert PUT /repositories/{id}/branch-protection
func (h *BranchProtectionHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	var req struct {
		Branch              string `json:"branch"`
		RequireReviews      bool   `json:"require_reviews"`
		RequiredApprovals   int    `json:"required_approvals"`
		DismissStaleReviews bool   `json:"dismiss_stale_reviews"`
		RequireStatusChecks bool   `json:"require_status_checks"`
		BlockForcePush      bool   `json:"block_force_push"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	if req.RequiredApprovals < 1 {
		req.RequiredApprovals = 1
	}

	rule := &model.BranchProtectionRule{
		RepositoryID:        repo.ID,
		Branch:              branch,
		RequireReviews:      req.RequireReviews,
		RequiredApprovals:   req.RequiredApprovals,
		DismissStaleReviews: req.DismissStaleReviews,
		RequireStatusChecks: req.RequireStatusChecks,
		BlockForcePush:      req.BlockForcePush,
	}
	if err := h.bpRepo.Upsert(r.Context(), rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save rule")
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Log(r.Context(), service.AuditEntry{
			ActorID: user.ID, ActorUsername: user.Username,
			Action: "update", ResourceType: "branch_protection",
			ResourceID: rule.ID.String(), ResourceName: branch,
			IPAddress: clientIP(r),
		})
	}
	writeJSON(w, http.StatusOK, rule)
}

// Delete DELETE /repositories/{id}/branch-protection/{branch}
func (h *BranchProtectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	branch := chi.URLParam(r, "branch")
	if err := h.bpRepo.Delete(r.Context(), repo.ID, branch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
