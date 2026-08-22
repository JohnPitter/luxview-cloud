package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CommunityPost struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Game        string    `json:"game"`
	GameName    string    `json:"game_name"`
	DisplayName string    `json:"display_name"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type CommunityMessage struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CommunityGamePlayers struct {
	AppID       string `json:"app_id"`
	Game        string `json:"game"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Players     int    `json:"players"`
	MaxPlayers  int    `json:"max_players"`
}

type CommunitySnapshot struct {
	PlayersOnline int                    `json:"players_online"`
	ChatOnline    int                    `json:"chat_online"`
	Games         []CommunityGamePlayers `json:"games"`
	Posts         []CommunityPost        `json:"posts"`
	Chat          []CommunityMessage     `json:"chat"`
}

func (a *App) CommunitySnapshot() (CommunitySnapshot, error) {
	var out CommunitySnapshot
	origin, err := platformOrigin()
	if err != nil {
		return out, err
	}
	resp, err := a.client.Get(origin + "/api/public/community")
	if err != nil {
		return out, fmt.Errorf("não consegui contatar a comunidade: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("comunidade indisponível (HTTP %d)", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return out, fmt.Errorf("resposta inválida da comunidade")
	}
	if out.Games == nil {
		out.Games = []CommunityGamePlayers{}
	}
	if out.Posts == nil {
		out.Posts = []CommunityPost{}
	}
	if out.Chat == nil {
		out.Chat = []CommunityMessage{}
	}
	return out, nil
}

func (a *App) CommunityHere() error {
	return a.communityAuthPost("/api/players/community/here", nil)
}

func (a *App) CommunitySend(body string) (CommunityMessage, error) {
	var out CommunityMessage
	origin, err := platformOrigin()
	if err != nil {
		return out, err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return out, fmt.Errorf("entre na conta LuxView")
	}
	payload, _ := json.Marshal(map[string]string{"body": body})
	req, err := http.NewRequest(http.MethodPost, origin+"/api/players/community/chat", bytes.NewReader(payload))
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return out, fmt.Errorf("não consegui enviar a mensagem: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("%s", stringsTrimError(raw))
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("resposta inválida")
	}
	return out, nil
}

func (a *App) communityAuthPost(path string, body any) error {
	origin, err := platformOrigin()
	if err != nil {
		return err
	}
	sess, err := loadPlayerSession()
	if err != nil || sess.Token == "" {
		return fmt.Errorf("entre na conta LuxView")
	}
	var reader io.Reader = http.NoBody
	if body != nil {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(http.MethodPost, origin+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s", stringsTrimError(raw))
	}
	return nil
}
