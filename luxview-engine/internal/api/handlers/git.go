package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

const (
	gitUploadPack  = "git-upload-pack"
	gitReceivePack = "git-receive-pack"
)

// GitHandler implements the Git HTTP smart protocol for hosted repositories.
type GitHandler struct {
	repositoryRepo *repository.RepositoryRepo
	repositorySvc  *service.RepositoryService
	pushHandler    *service.PushEventService
}

func NewGitHandler(repositoryRepo *repository.RepositoryRepo, repositorySvc *service.RepositoryService, pushHandler *service.PushEventService) *GitHandler {
	return &GitHandler{repositoryRepo: repositoryRepo, repositorySvc: repositorySvc, pushHandler: pushHandler}
}

// InfoRefs handles GET /{username}/{repo}.git/info/refs?service=git-{upload,receive}-pack
func (h *GitHandler) InfoRefs(w http.ResponseWriter, r *http.Request) {
	log := logger.With("git.info-refs")
	ctx := r.Context()

	repo, storagePath, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}

	svc := r.URL.Query().Get("service")
	if svc != gitUploadPack && svc != gitReceivePack {
		writeError(w, http.StatusForbidden, "unsupported service")
		return
	}

	if svc == gitReceivePack {
		userID := middleware.GetUserID(ctx)
		if userID == uuid.Nil || repo.UserID != userID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", svc))
	w.Header().Set("Cache-Control", "no-cache")

	// PKT-LINE with service announcement
	pkt := fmt.Sprintf("# service=%s\n", svc)
	fmt.Fprintf(w, "%04x%s0000", len(pkt)+4, pkt)

	cmd := exec.CommandContext(ctx, "git", strings.TrimPrefix(svc, "git-"), "--stateless-rpc", "--advertise-refs", storagePath)
	cmd.Stdout = w
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("svc", svc).Msg("info-refs failed")
	}
}

// UploadPack handles POST /{username}/{repo}.git/git-upload-pack (fetch/clone)
func (h *GitHandler) UploadPack(w http.ResponseWriter, r *http.Request) {
	log := logger.With("git.upload-pack")
	ctx := r.Context()

	_, storagePath, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	w.Header().Set("Cache-Control", "no-cache")

	cmd := exec.CommandContext(ctx, "git", "upload-pack", "--stateless-rpc", storagePath)
	cmd.Stdin = r.Body
	cmd.Stdout = w
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Msg("upload-pack failed")
	}
}

// ReceivePack handles POST /{username}/{repo}.git/git-receive-pack (push)
func (h *GitHandler) ReceivePack(w http.ResponseWriter, r *http.Request) {
	log := logger.With("git.receive-pack")
	ctx := r.Context()

	userID := middleware.GetUserID(ctx)
	repo, storagePath, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	if userID == uuid.Nil || repo.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Buffer the push output so we can parse refs after receive-pack succeeds.
	var out strings.Builder
	cmd := exec.CommandContext(ctx, "git", "receive-pack", "--stateless-rpc", storagePath)
	cmd.Stdin = r.Body
	cmd.Stdout = io.MultiWriter(w, &out)
	cmd.Stderr = io.Discard

	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	w.Header().Set("Cache-Control", "no-cache")

	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Msg("receive-pack failed")
		return
	}

	// Fire-and-forget push event (non-blocking).
	if h.pushHandler != nil {
		go func() {
			if err := h.pushHandler.HandlePush(ctx, repo.ID); err != nil {
				log.Error().Err(err).Str("repo", repo.Slug).Msg("push event handling failed")
			}
		}()
	}
}

// resolveRepo finds the repository from URL params {username}/{repo}.
// Public repos are accessible without authentication; private repos require the authenticated owner.
// Returns (repo, storagePath, true) on success, writes error and returns (nil, "", false) on failure.
func (h *GitHandler) resolveRepo(w http.ResponseWriter, r *http.Request) (*model.Repository, string, bool) {
	ctx := r.Context()
	username := chi.URLParam(r, "username")
	repoSlug := chi.URLParam(r, "repo")

	repo, err := h.repositoryRepo.FindByUsernameAndSlug(ctx, username, repoSlug)
	if err != nil || repo == nil {
		writeError(w, http.StatusNotFound, "repository not found")
		return nil, "", false
	}

	if repo.Visibility == model.RepositoryVisibilityPrivate {
		userID := middleware.GetUserID(ctx)
		if userID == uuid.Nil || repo.UserID != userID {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return nil, "", false
		}
	}

	return repo, repo.StoragePath, true
}
