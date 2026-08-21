package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/service"
)

type Players struct {
	players   *service.Player
	jwtSecret string
}

func NewPlayers(players *service.Player, jwtSecret string) *Players {
	return &Players{players: players, jwtSecret: jwtSecret}
}

type playerAuthBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Players) Register(w http.ResponseWriter, r *http.Request) {
	var body playerAuthBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	acct, err := h.players.Register(r.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeSession(w, r, http.StatusCreated, acct)
}

func (h *Players) Login(w http.ResponseWriter, r *http.Request) {
	var body playerAuthBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	acct, err := h.players.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	h.writeSession(w, r, http.StatusOK, acct)
}

func (h *Players) Me(w http.ResponseWriter, r *http.Request) {
	acct := middleware.GetPlayer(r.Context())
	if acct == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pub, err := h.players.Public(r.Context(), acct)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao carregar perfil")
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

func (h *Players) Link(w http.ResponseWriter, r *http.Request) {
	acct := middleware.GetPlayer(r.Context())
	if acct == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		AppID      string `json:"app_id"`
		TemplateID string `json:"template_id"`
		Nick       string `json:"nick"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	appID, err := uuid.Parse(body.AppID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "app inválido")
		return
	}
	link, err := h.players.Link(r.Context(), acct.ID, appID, body.TemplateID, body.Nick)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *Players) writeSession(w http.ResponseWriter, r *http.Request, status int, acct *model.PlayerAccount) {
	token, err := middleware.GeneratePlayerJWT(acct.ID, h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao emitir sessão")
		return
	}
	pub, err := h.players.Public(r.Context(), acct)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao carregar perfil")
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "player": pub})
}
