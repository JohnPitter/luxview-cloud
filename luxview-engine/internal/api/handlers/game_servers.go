package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

type GameServerHandler struct {
	appRepo        *repository.AppRepo
	gameConfigRepo *repository.GameServerConfigRepo
	gameServerSvc  *service.GameServerService
	serverIP       string
}

func NewGameServerHandler(
	appRepo *repository.AppRepo,
	gameConfigRepo *repository.GameServerConfigRepo,
	gameServerSvc *service.GameServerService,
	serverIP string,
) *GameServerHandler {
	return &GameServerHandler{
		appRepo:        appRepo,
		gameConfigRepo: gameConfigRepo,
		gameServerSvc:  gameServerSvc,
		serverIP:       serverIP,
	}
}

// ListTemplates returns all available game templates.
func (h *GameServerHandler) ListTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, service.GetGameTemplates())
}

// GetConfig returns the game server config for an app.
func (h *GameServerHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
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
	if app.AppType != model.AppTypeGame {
		writeError(w, http.StatusBadRequest, "app is not a game server")
		return
	}

	cfg, err := h.gameConfigRepo.GetByAppID(ctx, appID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return
	}

	// Attach template definition so the dashboard knows which fields to render
	tmpl := service.GetGameTemplate(cfg.TemplateID)
	type response struct {
		*model.GameServerConfig
		Template *model.GameTemplate `json:"template,omitempty"`
		ServerIP string              `json:"serverIp"`
	}
	writeJSON(w, http.StatusOK, response{cfg, tmpl, h.serverIP})
}

// UpdateConfig saves new game settings and restarts the container.
func (h *GameServerHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
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
	if app.AppType != model.AppTypeGame {
		writeError(w, http.StatusBadRequest, "app is not a game server")
		return
	}

	cfg, err := h.gameConfigRepo.GetByAppID(ctx, appID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return
	}

	var fields map[string]string
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cfg.ConfigFields = fields
	if err := h.gameConfigRepo.Update(ctx, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	// Restart container with updated env vars
	if app.ContainerID != "" && app.Status == model.AppStatusRunning {
		go h.gameServerSvc.Start(ctx, app, cfg) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, cfg)
}

// GetStatus queries live player count via A2S.
func (h *GameServerHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
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

	cfg, err := h.gameConfigRepo.GetByAppID(ctx, appID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return
	}

	// Query via internal Docker network (container name) so the engine
	// can reach the game server without hairpinning through the public IP.
	containerAddr := service.ContainerName(app.Subdomain)
	status, _ := h.gameServerSvc.QueryStatus(ctx, cfg, containerAddr)
	writeJSON(w, http.StatusOK, status)
}
