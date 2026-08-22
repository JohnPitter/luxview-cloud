package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type GameServerHandler struct {
	appRepo           *repository.AppRepo
	gameConfigRepo    *repository.GameServerConfigRepo
	gameServerSvc     *service.GameServerService
	gameClientStorage *service.ClientStore
	serverIP          string
	domain            string
	clientBaseZips    map[string]string // templateID -> base client zip path
}

func NewGameServerHandler(
	appRepo *repository.AppRepo,
	gameConfigRepo *repository.GameServerConfigRepo,
	gameServerSvc *service.GameServerService,
	gameClientStorage *service.ClientStore,
	serverIP string,
	domain string,
	clientBaseZips map[string]string,
) *GameServerHandler {
	return &GameServerHandler{
		appRepo:           appRepo,
		gameConfigRepo:    gameConfigRepo,
		gameServerSvc:     gameServerSvc,
		gameClientStorage: gameClientStorage,
		serverIP:          serverIP,
		domain:            domain,
		clientBaseZips:    clientBaseZips,
	}
}

// ListTemplates returns all available game templates.
func (h *GameServerHandler) ListTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, service.Templates())
}

func (h *GameServerHandler) loadGame(w http.ResponseWriter, r *http.Request) (*model.App, *model.GameServerConfig, bool) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
		return nil, nil, false
	}
	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return nil, nil, false
	}
	if app.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return nil, nil, false
	}
	if app.AppType != model.AppTypeGame {
		writeError(w, http.StatusBadRequest, "app is not a game server")
		return nil, nil, false
	}
	cfg, err := h.gameConfigRepo.GetByAppID(ctx, app.ID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return nil, nil, false
	}
	return app, cfg, true
}

func gameListed(cfg *model.GameServerConfig) bool {
	return strings.ToLower(cfg.ConfigFields["LUXVIEW_LISTED"]) == "true"
}

// GetConfig returns the game server config for an app.
func (h *GameServerHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	app, cfg, ok := h.loadGame(w, r)
	if !ok {
		return
	}

	// Attach the template and populate the global client selector for the dashboard.
	tmpl := h.configTemplate(cfg.TemplateID)
	responseConfig := cloneGameConfig(cfg)
	if h.gameClientStorage != nil && gameClientWithDownload[cfg.TemplateID] {
		if responseConfig.ConfigFields == nil {
			responseConfig.ConfigFields = map[string]string{}
		}
		if strings.TrimSpace(responseConfig.ConfigFields[model.GameClientGlobalFileField]) == "" {
			if key := h.gameClientStorage.DefaultGlobalKey(cfg.TemplateID); key != "" {
				responseConfig.ConfigFields[model.GameClientGlobalFileField] = key
			}
		}
	}
	type response struct {
		*model.GameServerConfig
		Template          *model.GameTemplate `json:"template,omitempty"`
		ServerIP          string              `json:"serverIp"`
		ClientDownloadURL string              `json:"clientDownloadUrl,omitempty"`
		ClientPublicURL   string              `json:"clientPublicUrl,omitempty"`
	}
	writeJSON(w, http.StatusOK, response{
		GameServerConfig:  responseConfig,
		Template:          tmpl,
		ServerIP:          h.serverIP,
		ClientDownloadURL: gameClientDownloadURL(app.ID.String(), cfg.TemplateID),
		ClientPublicURL:   gameClientPublicURL("https://"+h.domain, app.ID.String(), cfg.TemplateID),
	})
}

// UpdateConfig saves new game settings and restarts the container.
func (h *GameServerHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	app, cfg, ok := h.loadGame(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

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
	app, cfg, ok := h.loadGame(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.gameServerSvc.LiveStatus(r.Context(), app, cfg))
}

// GetPlayers lists online characters (name, class, map) from the game database.
func (h *GameServerHandler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	app, cfg, ok := h.loadGame(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	containerAddr := service.ContainerName(app.Subdomain)
	players, err := h.gameServerSvc.QueryPlayers(ctx, cfg, containerAddr)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, players)
}

// DownloadClient serves the per-server game client to the authenticated owner.
func (h *GameServerHandler) DownloadClient(w http.ResponseWriter, r *http.Request) {
	app, _, ok := h.loadGame(w, r)
	if !ok {
		return
	}
	h.serveGameClient(w, r, app)
}

// DownloadClientPublic serves the per-server game client over a public,
// unauthenticated link so players can share it with friends. No owner check —
// the client is meant to be distributed; it carries only the public server host.
func (h *GameServerHandler) DownloadClientPublic(w http.ResponseWriter, r *http.Request) {
	app := h.loadPublicListedGame(w, r)
	if app == nil {
		return
	}
	h.serveGameClient(w, r, app)
}

func (h *GameServerHandler) loadPublicListedGame(w http.ResponseWriter, r *http.Request) *model.App {
	ctx := r.Context()
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
		return nil
	}
	app, err := h.appRepo.FindByID(ctx, appID)
	if err != nil || app == nil {
		writeError(w, http.StatusNotFound, "app not found")
		return nil
	}
	cfg, err := h.gameConfigRepo.GetByAppID(ctx, app.ID)
	if err != nil || cfg == nil || !gameListed(cfg) {
		writeError(w, http.StatusNotFound, "app not found")
		return nil
	}
	return app
}

// DownloadClientPatchPublic streams only the per-server overlay (config, locale,
// launcher.config). The launcher uses this after the first install so a one-file
// change does not re-download hundreds of MiB on the same NIC as the game tick.
func (h *GameServerHandler) DownloadClientPatchPublic(w http.ResponseWriter, r *http.Request) {
	app := h.loadPublicListedGame(w, r)
	if app == nil {
		return
	}
	h.serveGameClientPatch(w, r, app)
}

// DownloadClientBasePublic serves the unmodified client zip (sendfile / Range).
// The launcher caches it by base_hash and only re-downloads when the zip itself changes.
func (h *GameServerHandler) DownloadClientBasePublic(w http.ResponseWriter, r *http.Request) {
	app := h.loadPublicListedGame(w, r)
	if app == nil {
		return
	}
	h.serveGameClientBase(w, r, app)
}

// PublicGameCard is one entry in the public launcher catalog.
type PublicGameCard struct {
	AppID       string `json:"app_id"`
	Name        string `json:"name"`         // server name (app.Name)
	Game        string `json:"game"`         // template id (e.g. "rakion")
	DisplayName string `json:"display_name"` // template display name
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`      // running + has a downloadable client
	DownloadURL string `json:"download_url"` // public, shareable client zip
	PatchURL    string `json:"patch_url,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	ServerIP    string `json:"server_ip"`
	AuthHost    string `json:"auth_host"` // <subdomain>.<domain> — onde o launcher faz login
	ClientHash  string `json:"client_hash,omitempty"`
	BaseHash    string `json:"base_hash,omitempty"`
}

// ListPublicGames returns the public catalog consumed by the LuxView launcher.
// Only game apps whose owner opted in (config_fields LUXVIEW_LISTED=true) are
// listed; no auth required. Disabled cards (not running / no client) render
// greyed-out in the launcher.
func (h *GameServerHandler) ListPublicGames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apps, err := h.appRepo.ListAllRunningOrError(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list games")
		return
	}
	cards := []PublicGameCard{}
	for i := range apps {
		app := apps[i]
		if app.AppType != model.AppTypeGame {
			continue
		}
		cfg, err := h.gameConfigRepo.GetByAppID(ctx, app.ID)
		if err != nil || cfg == nil {
			continue
		}
		if !gameListed(cfg) {
			continue
		}
		clientReady := true
		clientHash := ""
		baseHash := ""
		authHost := fmt.Sprintf("%s.%s", app.Subdomain, h.domain)
		if h.gameClientStorage != nil && gameClientWithDownload[cfg.TemplateID] {
			path, err := h.gameClientStorage.Resolve(ctx, app.ID, cfg.TemplateID, cfg.ConfigFields[model.GameClientGlobalFileField])
			if err != nil {
				clientReady = false
				log := logger.With("game-client-storage")
				log.Warn().Err(err).Str("app", app.Subdomain).Msg("launcher client is not ready")
			} else {
				fileHash, hashErr := h.gameClientStorage.FileHash(path)
				if hashErr != nil {
					log := logger.With("game-client-storage")
					log.Warn().Err(hashErr).Str("app", app.Subdomain).Msg("failed to hash launcher client")
				} else {
					baseHash = fileHash
					clientHash = clientRevision(cfg.TemplateID, fileHash, h.serverIP, authHost, cfg)
				}
			}
		}
		display, desc := cfg.TemplateID, ""
		if tmpl := service.Template(cfg.TemplateID); tmpl != nil {
			display = tmpl.DisplayName
			desc = tmpl.Description
		}
		origin := "https://" + h.domain
		appID := app.ID.String()
		patchURL, baseURL := "", ""
		if clientReady && gameClientWithDownload[cfg.TemplateID] {
			patchURL = gameClientPublicPatchURL(origin, appID, cfg.TemplateID)
			baseURL = gameClientPublicBaseURL(origin, appID, cfg.TemplateID)
		}
		cards = append(cards, PublicGameCard{
			AppID:       appID,
			Name:        app.Name,
			Game:        cfg.TemplateID,
			DisplayName: display,
			Description: desc,
			Enabled:     app.Status == model.AppStatusRunning && gameClientWithDownload[cfg.TemplateID] && clientReady,
			DownloadURL: gameClientPublicURL(origin, appID, cfg.TemplateID),
			PatchURL:    patchURL,
			BaseURL:     baseURL,
			ServerIP:    h.serverIP,
			AuthHost:    authHost,
			ClientHash:  clientHash,
			BaseHash:    baseHash,
		})
	}
	writeJSON(w, http.StatusOK, cards)
}

// serveGameClient generates and streams the configured client zip for app.
func (h *GameServerHandler) serveGameClient(w http.ResponseWriter, r *http.Request, app *model.App) {
	ctx := r.Context()
	if app.AppType != model.AppTypeGame {
		writeError(w, http.StatusBadRequest, "app is not a game server")
		return
	}

	cfg, err := h.gameConfigRepo.GetByAppID(ctx, app.ID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return
	}
	if h.serverIP == "" {
		writeError(w, http.StatusInternalServerError, "server IP is not configured")
		return
	}
	baseZipPath := h.clientBaseZips[cfg.TemplateID]
	if h.gameClientStorage != nil {
		baseZipPath, err = h.gameClientStorage.Resolve(ctx, app.ID, cfg.TemplateID, cfg.ConfigFields[model.GameClientGlobalFileField])
		if err != nil {
			writeError(w, http.StatusNotFound, "client is not available in global storage")
			return
		}
	}
	if baseZipPath == "" {
		writeError(w, http.StatusNotFound, "client download is not available for this template")
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

	// The client zip is large (hundreds of MB) and is streamed/generated on the
	// fly, so a slow client can take many minutes. Clear the server's write
	// deadline for this response so the connection isn't cut mid-stream
	// (otherwise the player sees "unexpected EOF").
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
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
			ServerIP: h.serverIP,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate Rakion client")
			return
		}
	case metin2TemplateID:
		if err := service.WriteLegacyMetin2ClientZip(baseZip, stat.Size(), w, service.LegacyMetin2ClientOptions{
			ServerIP:  h.serverIP,
			AuthPort:  cfg.GamePort,
			WorldPort: cfg.QueryPort,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate legacy Metin2 client")
			return
		}
	case tibiaTemplateID:
		if err := service.WriteTibiaClientZip(baseZip, stat.Size(), w, service.TibiaClientOptions{
			ServerName: app.Name,
			ServerIP:   h.serverIP,
			LoginPort:  tibiaLoginHTTPPort(cfg),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate Tibia client")
			return
		}
	case pristonTemplateID:
		serverName := app.Name
		if cfg.ConfigFields != nil {
			if name := strings.TrimSpace(cfg.ConfigFields["PRISTON_SERVER_NAME"]); name != "" {
				serverName = name
			}
		}
		if err := service.WritePristonClientZip(baseZip, stat.Size(), w, service.PristonClientOptions{
			ServerName: serverName,
			ServerIP:   h.serverIP,
			GamePort:   cfg.GamePort,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate Priston client")
			return
		}
	default: // openmu, muemu — launcher.config with ConnectServer IP/port
		if err := service.WriteOpenMUClientZip(baseZip, stat.Size(), w, service.OpenMUClientOptions{
			ServerName: app.Name,
			ServerIP:   h.serverIP,
			GamePort:   cfg.GamePort,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate MU client")
			return
		}
	}
}

func (h *GameServerHandler) serveGameClientPatch(w http.ResponseWriter, r *http.Request, app *model.App) {
	cfg, baseZip, stat, ok := h.openClientBaseZip(w, r, app)
	if !ok {
		return
	}
	defer baseZip.Close()

	clearClientWriteDeadline(w)

	var buf bytes.Buffer
	var err error
	switch cfg.TemplateID {
	case rakionTemplateID:
		err = service.WriteRakionClientPatch(baseZip, stat.Size(), &buf, service.RakionClientOptions{
			AuthHost: fmt.Sprintf("%s.%s", app.Subdomain, h.domain),
			ServerIP: h.serverIP,
		})
	case metin2TemplateID:
		err = service.WriteLegacyMetin2ClientPatch(baseZip, stat.Size(), &buf, service.LegacyMetin2ClientOptions{
			ServerIP:  h.serverIP,
			AuthPort:  cfg.GamePort,
			WorldPort: cfg.QueryPort,
		})
	case tibiaTemplateID:
		err = service.WriteTibiaClientPatch(baseZip, stat.Size(), &buf, service.TibiaClientOptions{
			ServerName: app.Name,
			ServerIP:   h.serverIP,
			LoginPort:  tibiaLoginHTTPPort(cfg),
		})
	case pristonTemplateID:
		err = service.WritePristonClientPatch(baseZip, stat.Size(), &buf, service.PristonClientOptions{
			ServerName: pristonClientServerName(app, cfg),
			ServerIP:   h.serverIP,
			GamePort:   cfg.GamePort,
		})
	default:
		err = service.WriteOpenMUClientPatch(&buf, service.OpenMUClientOptions{
			ServerName: app.Name,
			ServerIP:   h.serverIP,
			GamePort:   cfg.GamePort,
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate client patch")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-client-patch.zip", app.Subdomain))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

func (h *GameServerHandler) serveGameClientBase(w http.ResponseWriter, r *http.Request, app *model.App) {
	_, baseZip, stat, ok := h.openClientBaseZip(w, r, app)
	if !ok {
		return
	}
	defer baseZip.Close()

	clearClientWriteDeadline(w)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-client-base.zip", app.Subdomain))
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), baseZip)
}

func (h *GameServerHandler) openClientBaseZip(w http.ResponseWriter, r *http.Request, app *model.App) (*model.GameServerConfig, *os.File, os.FileInfo, bool) {
	ctx := r.Context()
	if app.AppType != model.AppTypeGame {
		writeError(w, http.StatusBadRequest, "app is not a game server")
		return nil, nil, nil, false
	}
	cfg, err := h.gameConfigRepo.GetByAppID(ctx, app.ID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return nil, nil, nil, false
	}
	if h.serverIP == "" {
		writeError(w, http.StatusInternalServerError, "server IP is not configured")
		return nil, nil, nil, false
	}
	baseZipPath := h.clientBaseZips[cfg.TemplateID]
	if h.gameClientStorage != nil {
		baseZipPath, err = h.gameClientStorage.Resolve(ctx, app.ID, cfg.TemplateID, cfg.ConfigFields[model.GameClientGlobalFileField])
		if err != nil {
			writeError(w, http.StatusNotFound, "client is not available in global storage")
			return nil, nil, nil, false
		}
	}
	if baseZipPath == "" {
		writeError(w, http.StatusNotFound, "client download is not available for this template")
		return nil, nil, nil, false
	}
	baseZip, err := os.Open(baseZipPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "client base zip not found")
		return nil, nil, nil, false
	}
	stat, err := baseZip.Stat()
	if err != nil {
		baseZip.Close()
		writeError(w, http.StatusInternalServerError, "failed to read client base zip")
		return nil, nil, nil, false
	}
	return cfg, baseZip, stat, true
}

func clearClientWriteDeadline(w http.ResponseWriter) {
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
}

func pristonClientServerName(app *model.App, cfg *model.GameServerConfig) string {
	if cfg != nil && cfg.ConfigFields != nil {
		if name := strings.TrimSpace(cfg.ConfigFields["PRISTON_SERVER_NAME"]); name != "" {
			return name
		}
	}
	return app.Name
}

func (h *GameServerHandler) configTemplate(templateID string) *model.GameTemplate {
	tmpl := service.Template(templateID)
	if tmpl == nil || h.gameClientStorage == nil || !gameClientWithDownload[templateID] {
		return tmpl
	}

	options, err := h.gameClientStorage.ListGlobalFiles()
	if err != nil {
		log := logger.With("game-client-storage")
		log.Warn().Err(err).Str("template", templateID).Msg("failed to list global client files")
		return tmpl
	}
	copyTemplate := *tmpl
	copyTemplate.ConfigFields = make([]model.ConfigFieldDef, len(tmpl.ConfigFields))
	copy(copyTemplate.ConfigFields, tmpl.ConfigFields)
	for i := range copyTemplate.ConfigFields {
		field := &copyTemplate.ConfigFields[i]
		if field.Key == model.GameClientGlobalFileField {
			field.Options = options
		}
	}
	return &copyTemplate
}

func cloneGameConfig(cfg *model.GameServerConfig) *model.GameServerConfig {
	copyConfig := *cfg
	copyConfig.ConfigFields = make(map[string]string, len(cfg.ConfigFields))
	for key, value := range cfg.ConfigFields {
		copyConfig.ConfigFields[key] = value
	}
	return &copyConfig
}

const (
	openMUTemplateID  = "openmu"
	muemuTemplateID   = "muemu"
	rakionTemplateID  = "rakion"
	metin2TemplateID  = "metin2"
	tibiaTemplateID   = "tibia"
	pristonTemplateID = "priston"
)

// gameClientWithDownload lists templates that offer a configured client download.
var gameClientWithDownload = map[string]bool{
	openMUTemplateID:  true,
	muemuTemplateID:   true,
	rakionTemplateID:  true,
	metin2TemplateID:  true,
	tibiaTemplateID:   true,
	pristonTemplateID: true,
}

func gameClientDownloadURL(appID string, templateID string) string {
	if !gameClientWithDownload[templateID] {
		return ""
	}
	return "/api/apps/" + appID + "/game-client/download"
}

// gameClientPublicURL is the shareable, unauthenticated client download link
// players can pass to friends. baseURL is the platform origin (e.g.
// https://luxview.cloud).
func gameClientPublicURL(baseURL, appID, templateID string) string {
	if !gameClientWithDownload[templateID] {
		return ""
	}
	return baseURL + "/api/public/game-client/" + appID
}

func gameClientPublicPatchURL(baseURL, appID, templateID string) string {
	if u := gameClientPublicURL(baseURL, appID, templateID); u != "" {
		return u + "/patch"
	}
	return ""
}

func gameClientPublicBaseURL(baseURL, appID, templateID string) string {
	if u := gameClientPublicURL(baseURL, appID, templateID); u != "" {
		return u + "/base"
	}
	return ""
}

// clientRevision fingerprints the zip the launcher would download: the base
// client file plus the values patched into that zip at download time.
func clientRevision(templateID, fileHash, serverIP, authHost string, cfg *model.GameServerConfig) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%s|%s|%s", fileHash, templateID, serverIP)
	switch templateID {
	case rakionTemplateID:
		fmt.Fprintf(sum, "|%s", authHost)
	case metin2TemplateID:
		fmt.Fprintf(sum, "|%d|%d", cfg.GamePort, cfg.QueryPort)
	case tibiaTemplateID:
		fmt.Fprintf(sum, "|%d", tibiaLoginHTTPPort(cfg))
	case openMUTemplateID, muemuTemplateID, pristonTemplateID:
		fmt.Fprintf(sum, "|%d", cfg.GamePort)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func staticGameServerStatus(app *model.App, tmpl *model.GameTemplate) *model.GameServerStatus {
	if tmpl == nil || tmpl.SupportsQuery {
		return nil
	}
	return &model.GameServerStatus{Running: app.Status == model.AppStatusRunning}
}

// tibiaLoginHTTPPort é a porta publicada do login HTTP (login-server) que o
// client OTClient usa para autenticar. Padrão 8088 quando o extra port não
// estiver persistido.
func tibiaLoginHTTPPort(cfg *model.GameServerConfig) int {
	for _, ep := range cfg.ExtraPorts {
		if ep.Port == 8088 {
			return ep.Port
		}
	}
	return 8088
}
