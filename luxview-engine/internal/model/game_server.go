package model

import (
	"time"

	"github.com/google/uuid"
)

type GameServerConfig struct {
	ID           uuid.UUID         `json:"id"`
	AppID        uuid.UUID         `json:"app_id"`
	TemplateID   string            `json:"template_id"`
	Image        string            `json:"image"`
	GamePort     int               `json:"game_port"`
	QueryPort    int               `json:"query_port,omitempty"`
	DataDir      string            `json:"data_dir"`
	DataVolume   string            `json:"data_volume,omitempty"` // custom Docker volume name; defaults to luxview-game-{subdomain}-data
	Protocol     string            `json:"protocol"`
	ConfigFields map[string]string `json:"config_fields"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type GameServerStatus struct {
	Running    bool `json:"running"`
	Players    int  `json:"players"`
	MaxPlayers int  `json:"max_players"`
}

type SelectOptionDef struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ConfigFieldDef struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Type        string            `json:"type"` // "text", "password", "number", "select"
	Options     []SelectOptionDef `json:"options,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Section     string            `json:"section,omitempty"`
}

type GameTemplate struct {
	ID               string           `json:"id"`
	DisplayName      string           `json:"display_name"`
	Description      string           `json:"description"`
	Protocol         string           `json:"protocol"`
	DefaultGamePort  int              `json:"default_game_port"`
	DefaultQueryPort int              `json:"default_query_port"`
	DefaultImage     string           `json:"default_image"`
	ConfigFields     []ConfigFieldDef `json:"config_fields"`
	SupportsQuery    bool             `json:"supports_query"`
}
