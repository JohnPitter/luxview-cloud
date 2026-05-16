package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/service"
)

type GitHubHandler struct {
	githubAppSvc *service.GitHubAppService
}

func NewGitHubHandler(githubAppSvc *service.GitHubAppService) *GitHubHandler {
	return &GitHubHandler{githubAppSvc: githubAppSvc}
}

func (h *GitHubHandler) CreateRepo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CreateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	repo, err := h.githubAppSvc.CreateRepo(ctx, user, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, repo)
}

func (h *GitHubHandler) CommitWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CommitWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Owner == "" || req.Repo == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "owner, repo and content are required")
		return
	}

	if err := h.githubAppSvc.CommitWorkflow(ctx, user, req); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "workflow committed"})
}

func (h *GitHubHandler) SyncSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := middleware.GetUser(ctx)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.SyncSecretsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Owner == "" || req.Repo == "" {
		writeError(w, http.StatusBadRequest, "owner and repo are required")
		return
	}

	if err := h.githubAppSvc.SyncSecretsToGitHub(ctx, user, req); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "secrets synced"})
}
