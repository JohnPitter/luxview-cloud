package handlers

import (
	"net/http"

	"github.com/luxview/engine/internal/service"
)

type TraefikHandler struct {
	router *service.RouterService
}

func NewTraefikHandler(router *service.RouterService) *TraefikHandler {
	return &TraefikHandler{router: router}
}

// GetConfig returns the dynamic Traefik configuration.
func (h *TraefikHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	config, err := h.router.GenerateConfig(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate config")
		return
	}

	writeJSON(w, http.StatusOK, config)
}
