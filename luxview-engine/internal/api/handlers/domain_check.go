package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

// DomainCheckHandler diagnoses a custom domain's DNS + cert state.
type DomainCheckHandler struct {
	appRepo *repository.AppRepo
	checker *service.DomainChecker
}

func NewDomainCheckHandler(appRepo *repository.AppRepo, checker *service.DomainChecker) *DomainCheckHandler {
	return &DomainCheckHandler{appRepo: appRepo, checker: checker}
}

// Check responds to GET /api/apps/{id}/domain-check
func (h *DomainCheckHandler) Check(w http.ResponseWriter, r *http.Request) {
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

	// Allow caller to probe a domain that hasn't been saved yet via ?domain=...
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" && app.CustomDomain != nil {
		domain = *app.CustomDomain
	}
	if domain == "" {
		writeError(w, http.StatusBadRequest, "no domain to check")
		return
	}

	result := h.checker.Check(ctx, domain)
	writeJSON(w, http.StatusOK, result)
}
