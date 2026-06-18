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

type IssueHandler struct {
	repositoryRepo *repository.RepositoryRepo
	issueSvc       *service.IssueService
	auditSvc       *service.AuditService
}

func NewIssueHandler(repositoryRepo *repository.RepositoryRepo, issueSvc *service.IssueService, auditSvc *service.AuditService) *IssueHandler {
	return &IssueHandler{repositoryRepo: repositoryRepo, issueSvc: issueSvc, auditSvc: auditSvc}
}

func (h *IssueHandler) authorizeRepository(w http.ResponseWriter, r *http.Request) (*model.Repository, bool) {
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

func (h *IssueHandler) issueNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "number"))
	if err != nil || n < 1 {
		writeError(w, http.StatusBadRequest, "invalid issue number")
		return 0, false
	}
	return n, true
}

func parseLabelIDs(raw []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// List GET /repositories/{id}/issues?status=open
func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
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
	issues, total, err := h.issueSvc.List(r.Context(), repo.ID, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issues")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues, "total": total})
}

// Create POST /repositories/{id}/issues
func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())

	var req struct {
		Title    string   `json:"title"`
		Body     string   `json:"body"`
		LabelIDs []string `json:"label_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	issue, err := h.issueSvc.Create(r.Context(), service.CreateIssueRequest{
		RepositoryID: repo.ID,
		AuthorID:     user.ID,
		Title:        req.Title,
		Body:         req.Body,
		LabelIDs:     parseLabelIDs(req.LabelIDs),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.auditSvc != nil {
		h.auditSvc.Log(r.Context(), service.AuditEntry{
			ActorID: user.ID, ActorUsername: user.Username,
			Action: "create", ResourceType: "issue",
			ResourceID: issue.ID.String(), ResourceName: issue.Title,
			IPAddress: clientIP(r),
		})
	}
	writeJSON(w, http.StatusCreated, issue)
}

// Get GET /repositories/{id}/issues/{number}
func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.issueNumber(w, r)
	if !ok {
		return
	}
	issue, err := h.issueSvc.Get(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

// Update PATCH /repositories/{id}/issues/{number}
func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.issueNumber(w, r)
	if !ok {
		return
	}
	var req struct {
		Title    string    `json:"title"`
		Body     string    `json:"body"`
		LabelIDs *[]string `json:"label_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var labelIDs []uuid.UUID
	setLabels := req.LabelIDs != nil
	if setLabels {
		labelIDs = parseLabelIDs(*req.LabelIDs)
	}
	issue, err := h.issueSvc.Update(r.Context(), repo.ID, number, req.Title, req.Body, labelIDs, setLabels)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

// SetStatus POST /repositories/{id}/issues/{number}/status
func (h *IssueHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.issueNumber(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())
	var req struct {
		Status model.IssueStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status != model.IssueStatusOpen && req.Status != model.IssueStatusClosed {
		writeError(w, http.StatusBadRequest, "status must be 'open' or 'closed'")
		return
	}
	issue, err := h.issueSvc.SetStatus(r.Context(), repo.ID, number, req.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.auditSvc != nil {
		h.auditSvc.Log(r.Context(), service.AuditEntry{
			ActorID: user.ID, ActorUsername: user.Username,
			Action: "update", ResourceType: "issue",
			ResourceID: issue.ID.String(), ResourceName: string(req.Status),
			IPAddress: clientIP(r),
		})
	}
	writeJSON(w, http.StatusOK, issue)
}

// Labels

// ListLabels GET /repositories/{id}/labels
func (h *IssueHandler) ListLabels(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	labels, err := h.issueSvc.ListLabels(r.Context(), repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"labels": labels})
}

// CreateLabel POST /repositories/{id}/labels
func (h *IssueHandler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	label, err := h.issueSvc.CreateLabel(r.Context(), repo.ID, req.Name, req.Color, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, label)
}

// DeleteLabel DELETE /repositories/{id}/labels/{labelId}
func (h *IssueHandler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	labelID, err := uuid.Parse(chi.URLParam(r, "labelId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid label ID")
		return
	}
	if err := h.issueSvc.DeleteLabel(r.Context(), repo.ID, labelID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Comments

// ListComments GET /repositories/{id}/issues/{number}/comments
func (h *IssueHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.issueNumber(w, r)
	if !ok {
		return
	}
	issue, err := h.issueSvc.Get(r.Context(), repo.ID, number)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	comments, err := h.issueSvc.ListComments(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list comments")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

// AddComment POST /repositories/{id}/issues/{number}/comments
func (h *IssueHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.authorizeRepository(w, r)
	if !ok {
		return
	}
	number, ok := h.issueNumber(w, r)
	if !ok {
		return
	}
	user := middleware.GetUser(r.Context())
	issue, err := h.issueSvc.Get(r.Context(), repo.ID, number)
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
	comment, err := h.issueSvc.AddComment(r.Context(), issue.ID, user.ID, req.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// DeleteComment DELETE /repositories/{id}/issues/{number}/comments/{commentId}
func (h *IssueHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorizeRepository(w, r); !ok {
		return
	}
	user := middleware.GetUser(r.Context())
	commentID, err := uuid.Parse(chi.URLParam(r, "commentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}
	if err := h.issueSvc.DeleteComment(r.Context(), commentID, user.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
