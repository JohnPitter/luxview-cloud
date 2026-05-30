package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type GameServerHandler struct {
	appRepo        *repository.AppRepo
	gameConfigRepo *repository.GameServerConfigRepo
	gameServerSvc  *service.GameServerService
	serverIP       string
	domain         string
	clientBaseZips map[string]string // templateID -> base client zip path
}

func NewGameServerHandler(
	appRepo *repository.AppRepo,
	gameConfigRepo *repository.GameServerConfigRepo,
	gameServerSvc *service.GameServerService,
	serverIP string,
	domain string,
	clientBaseZips map[string]string,
) *GameServerHandler {
	return &GameServerHandler{
		appRepo:        appRepo,
		gameConfigRepo: gameConfigRepo,
		gameServerSvc:  gameServerSvc,
		serverIP:       serverIP,
		domain:         domain,
		clientBaseZips: clientBaseZips,
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
		Template          *model.GameTemplate `json:"template,omitempty"`
		ServerIP          string              `json:"serverIp"`
		ClientDownloadURL string              `json:"clientDownloadUrl,omitempty"`
	}
	writeJSON(w, http.StatusOK, response{
		GameServerConfig:  cfg,
		Template:          tmpl,
		ServerIP:          h.serverIP,
		ClientDownloadURL: gameClientDownloadURL(appID.String(), cfg.TemplateID),
	})
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

	var body struct {
		ConfigFields map[string]string `json:"config_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ConfigFields == nil {
		writeError(w, http.StatusBadRequest, "config_fields is required")
		return
	}

	cfg.ConfigFields = body.ConfigFields
	if err := h.gameConfigRepo.Update(ctx, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	// Restart container with updated env vars; keep DB container_id in sync with the new container.
	// Use a fresh background context — request context is canceled as soon as we writeJSON below,
	// which would abort the docker stop/remove/create chain mid-flight.
	if app.Status == model.AppStatusRunning {
		log := logger.With("game-server")
		go func() {
			bgCtx := context.Background()
			containerID, startErr := h.gameServerSvc.Start(bgCtx, app, cfg)
			status := model.AppStatusRunning
			if startErr != nil {
				log.Error().Err(startErr).Str("app", app.Subdomain).Msg("game server restart failed")
				status = model.AppStatusError
				containerID = app.ContainerID
			} else {
				log.Info().Str("app", app.Subdomain).Str("container", containerID[:12]).Msg("game server restarted with new config")
			}
			_ = h.appRepo.UpdateStatus(bgCtx, app.ID, status, containerID)
		}()
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

	// OpenMU has no A2S query protocol; estimate online players by counting
	// established connections on its game-server ports inside the container.
	if cfg.TemplateID == openMUTemplateID {
		writeJSON(w, http.StatusOK, h.openMUStatus(ctx, app, cfg))
		return
	}

	if status := staticGameServerStatus(app, service.GetGameTemplate(cfg.TemplateID)); status != nil {
		writeJSON(w, http.StatusOK, status)
		return
	}

	// Query via internal Docker network (container name) so the engine
	// can reach the game server without hairpinning through the public IP.
	containerAddr := service.ContainerName(app.Subdomain)
	status, _ := h.gameServerSvc.QueryStatus(ctx, cfg, containerAddr)
	writeJSON(w, http.StatusOK, status)
}

// GetPlayers returns the list of connected players via A2S_PLAYER.
func (h *GameServerHandler) GetPlayers(w http.ResponseWriter, r *http.Request) {
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

	containerAddr := service.ContainerName(app.Subdomain)
	players, err := h.gameServerSvc.QueryPlayers(ctx, cfg, containerAddr)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, players)
}

func (h *GameServerHandler) DownloadClient(w http.ResponseWriter, r *http.Request) {
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
	baseZipPath := h.clientBaseZips[cfg.TemplateID]
	if baseZipPath == "" {
		writeError(w, http.StatusNotFound, "client download is not available for this template")
		return
	}
	if h.serverIP == "" {
		writeError(w, http.StatusInternalServerError, "server IP is not configured")
		return
	}

	baseZip, err := os.Open(baseZipPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "client base zip not found")
		return
	}
	defer baseZip.Close()

	stat, err := baseZip.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read client base zip")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-client.zip", app.Subdomain))

	switch cfg.TemplateID {
	case rakionTemplateID:
		// Rakion's client reaches the auth web at the server's subdomain; the
		// injected config.xfs points there (served via Traefik/HTTPS).
		authHost := fmt.Sprintf("%s.%s", app.Subdomain, h.domain)
		if err := service.WriteRakionClientZip(baseZip, stat.Size(), w, service.RakionClientOptions{
			AuthHost: authHost,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate Rakion client")
			return
		}
	default: // openMUTemplateID
		if err := service.WriteOpenMUClientZip(baseZip, stat.Size(), w, service.OpenMUClientOptions{
			ServerName: app.Name,
			ServerIP:   h.serverIP,
			GamePort:   cfg.GamePort,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate OpenMU client")
			return
		}
	}
}

const (
	openMUTemplateID = "openmu"
	rakionTemplateID = "rakion"
)

// gameClientWithDownload lists templates that offer a configured client download.
var gameClientWithDownload = map[string]bool{
	openMUTemplateID: true,
	rakionTemplateID: true,
}

func gameClientDownloadURL(appID string, templateID string) string {
	if !gameClientWithDownload[templateID] {
		return ""
	}
	return "/api/apps/" + appID + "/game-client/download"
}

func staticGameServerStatus(app *model.App, tmpl *model.GameTemplate) *model.GameServerStatus {
	if tmpl == nil || tmpl.SupportsQuery {
		return nil
	}
	return &model.GameServerStatus{Running: app.Status == model.AppStatusRunning}
}

// openMUStatus reports the OpenMU server status, estimating the online player
// count from established connections on the game-server ports (OpenMU has no
// query protocol). Uses the container name so it survives container recreation.
func (h *GameServerHandler) openMUStatus(ctx context.Context, app *model.App, cfg *model.GameServerConfig) *model.GameServerStatus {
	status := &model.GameServerStatus{Running: app.Status == model.AppStatusRunning}
	if !status.Running {
		return status
	}
	status.MaxPlayers = openMUMaxPlayers(cfg)
	if n, err := h.gameServerSvc.CountConnections(ctx, service.ContainerName(app.Subdomain), openMUGamePorts(cfg)); err == nil {
		status.Players = n
	}
	return status
}

// openMUGamePorts is the set of ports players hold a connection on while in-game:
// the main game port (QueryPort) plus the extra "GameServer" ports.
func openMUGamePorts(cfg *model.GameServerConfig) map[int]bool {
	ports := make(map[int]bool)
	if cfg.QueryPort > 0 {
		ports[cfg.QueryPort] = true
	}
	for _, ep := range cfg.ExtraPorts {
		if strings.Contains(strings.ToLower(ep.Label), "gameserver") {
			ports[ep.Port] = true
		}
	}
	return ports
}

func openMUMaxPlayers(cfg *model.GameServerConfig) int {
	if v := cfg.ConfigFields["OPENMU_MAX_CONNECTIONS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1000
}
