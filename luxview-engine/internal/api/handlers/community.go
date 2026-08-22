package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
)

type Community struct {
	posts      *repository.CommunityRepo
	apps       *repository.AppRepo
	gameCfg    *repository.GameServerConfigRepo
	gameServer *service.GameServerService
	hub        *service.CommunityHub
}

func NewCommunity(
	posts *repository.CommunityRepo,
	apps *repository.AppRepo,
	gameCfg *repository.GameServerConfigRepo,
	gameServer *service.GameServerService,
	hub *service.CommunityHub,
) *Community {
	return &Community{posts: posts, apps: apps, gameCfg: gameCfg, gameServer: gameServer, hub: hub}
}

func (h *Community) Snapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	posts, err := h.posts.ListFeed(ctx, 40)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao carregar o feed")
		return
	}
	for i := range posts {
		decoratePost(&posts[i])
	}
	chat, err := h.posts.ListChat(ctx, 80)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao carregar o chat")
		return
	}
	games := h.listedPlayerCounts(r)
	total := 0
	for _, g := range games {
		total += g.Players
	}
	writeJSON(w, http.StatusOK, model.CommunitySnapshot{
		PlayersOnline: total,
		ChatOnline:    h.hub.OnlineCount(),
		Games:         games,
		Posts:         posts,
		Chat:          chat,
	})
}

func (h *Community) Here(w http.ResponseWriter, r *http.Request) {
	acct := middleware.GetPlayer(r.Context())
	if acct == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	h.hub.Touch(acct.ID, acct.Username)
	writeJSON(w, http.StatusOK, map[string]int{"chat_online": h.hub.OnlineCount()})
}

func (h *Community) SendChat(w http.ResponseWriter, r *http.Request) {
	acct := middleware.GetPlayer(r.Context())
	if acct == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	text, err := service.NormalizeCommunityText(body.Body, service.CommunityChatMax)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.hub.AllowChat(acct.ID); err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	h.hub.Touch(acct.ID, acct.Username)
	msg, err := h.posts.InsertChat(r.Context(), acct.ID, acct.Username, text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível enviar")
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (h *Community) ListPosts(w http.ResponseWriter, r *http.Request) {
	app, _, ok := h.ownerGame(w, r)
	if !ok {
		return
	}
	posts, err := h.posts.ListByApp(r.Context(), app.ID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao listar postagens")
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

func (h *Community) CreatePost(w http.ResponseWriter, r *http.Request) {
	app, cfg, ok := h.ownerGame(w, r)
	if !ok {
		return
	}
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	title, err := service.NormalizeCommunityText(body.Title, service.CommunityTitleMax)
	if err != nil {
		writeError(w, http.StatusBadRequest, "título inválido")
		return
	}
	text, err := service.NormalizeCommunityText(body.Body, service.CommunityBodyMax)
	if err != nil {
		writeError(w, http.StatusBadRequest, "texto inválido")
		return
	}
	post := &model.CommunityPost{AppID: app.ID, Title: title, Body: text}
	if err := h.posts.CreatePost(r.Context(), post, middleware.GetUserID(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível publicar")
		return
	}
	post.Game = cfg.TemplateID
	post.GameName = app.Name
	decoratePost(post)
	writeJSON(w, http.StatusCreated, post)
}

func (h *Community) DeletePost(w http.ResponseWriter, r *http.Request) {
	app, _, ok := h.ownerGame(w, r)
	if !ok {
		return
	}
	postID, err := uuid.Parse(chi.URLParam(r, "postId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "post inválido")
		return
	}
	if err := h.posts.DeletePost(r.Context(), postID, app.ID); err != nil {
		writeError(w, http.StatusNotFound, "postagem não encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Community) ownerGame(w http.ResponseWriter, r *http.Request) (*model.App, *model.GameServerConfig, bool) {
	ctx := r.Context()
	userID := middleware.GetUserID(ctx)
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid app id")
		return nil, nil, false
	}
	app, err := h.apps.FindByID(ctx, appID)
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
	cfg, err := h.gameCfg.GetByAppID(ctx, app.ID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "game config not found")
		return nil, nil, false
	}
	return app, cfg, true
}

func (h *Community) listedPlayerCounts(r *http.Request) []model.CommunityGamePlayers {
	ctx := r.Context()
	apps, err := h.apps.ListAllRunningOrError(ctx)
	if err != nil {
		return []model.CommunityGamePlayers{}
	}
	type job struct {
		app model.App
		cfg *model.GameServerConfig
	}
	var jobs []job
	for i := range apps {
		app := apps[i]
		if app.AppType != model.AppTypeGame {
			continue
		}
		cfg, err := h.gameCfg.GetByAppID(ctx, app.ID)
		if err != nil || cfg == nil || !gameListed(cfg) {
			continue
		}
		jobs = append(jobs, job{app: app, cfg: cfg})
	}
	out := make([]model.CommunityGamePlayers, len(jobs))
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			app, cfg := jobs[i].app, jobs[i].cfg
			display := cfg.TemplateID
			if tmpl := service.Template(cfg.TemplateID); tmpl != nil {
				display = tmpl.DisplayName
			}
			st := h.gameServer.LiveStatus(ctx, &app, cfg)
			out[i] = model.CommunityGamePlayers{
				AppID:       app.ID.String(),
				Game:        cfg.TemplateID,
				Name:        app.Name,
				DisplayName: display,
				Players:     st.Players,
				MaxPlayers:  st.MaxPlayers,
			}
		}(i)
	}
	wg.Wait()
	if out == nil {
		out = []model.CommunityGamePlayers{}
	}
	return out
}

func decoratePost(p *model.CommunityPost) {
	if tmpl := service.Template(p.Game); tmpl != nil {
		p.DisplayName = tmpl.DisplayName
		return
	}
	if p.DisplayName == "" {
		p.DisplayName = p.GameName
	}
}
