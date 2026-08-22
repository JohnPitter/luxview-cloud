package model

import (
	"time"

	"github.com/google/uuid"
)

type CommunityPost struct {
	ID          uuid.UUID `json:"id"`
	AppID       uuid.UUID `json:"app_id"`
	Game        string    `json:"game"`
	GameName    string    `json:"game_name"`
	DisplayName string    `json:"display_name"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type CommunityMessage struct {
	ID        uuid.UUID `json:"id"`
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
