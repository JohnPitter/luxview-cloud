package service

import (
	"strings"

	"github.com/luxview/engine/internal/model"
)

// OpenMUServerGroup is one physical MU server in the launcher picker
// (ConnectServer group = ServerID/20) plus the channels it exposes.
type OpenMUServerGroup struct {
	Group      int                 `json:"group"`
	Name       string              `json:"name"`
	Difficulty string              `json:"difficulty"`
	Channels   []OpenMUChannelMeta `json:"channels,omitempty"`
}

// OpenMUChannelMeta is a known ConnectServer ServerID and its PvP/PvE badge.
type OpenMUChannelMeta struct {
	ID   int    `json:"id"`
	Mode string `json:"mode"`
}

type openMUGroupDefaults struct {
	serverID   int
	nameKey    string
	diffKey    string
	name       string
	difficulty string
	channels   []OpenMUChannelMeta
}

// Defaults match the live ConnectServer layout:
//
//	0–19  Season 6 Pt 3 (Easy) — ServerID 0 = Canal 1 PvP
//	20–39 99d (Hard)           — ServerID 20 = Canal 1 PvP, 21 = Canal 2 PvE
//	40–59 Season 2 (Medium)    — ServerID 40 = Canal 1 PvP
var openMUGroupCatalog = []openMUGroupDefaults{
	{serverID: 20, nameKey: "OPENMU_S20_NAME", diffKey: "OPENMU_S20_DIFFICULTY", name: "99d", difficulty: "Hard", channels: []OpenMUChannelMeta{{ID: 20, Mode: "PvP"}, {ID: 21, Mode: "PvE"}}},
	{serverID: 40, nameKey: "OPENMU_S40_NAME", diffKey: "OPENMU_S40_DIFFICULTY", name: "Season 2", difficulty: "Medium", channels: []OpenMUChannelMeta{{ID: 40, Mode: "PvP"}}},
	{serverID: 0, nameKey: "OPENMU_S0_NAME", diffKey: "OPENMU_S0_DIFFICULTY", name: "Season 6 Pt 3", difficulty: "Easy", channels: []OpenMUChannelMeta{{ID: 0, Mode: "PvP"}}},
}

const openMUChannel21ModeKey = "OPENMU_S21_MODE"

func withOpenMUChannelCatalog(t model.GameTemplate) model.GameTemplate {
	t.ConfigFields = append(t.ConfigFields, openmuChannelCatalogFields()...)
	return t
}

func openmuChannelCatalogFields() []model.ConfigFieldDef {
	section := "Launcher — servidores e canais"
	diffOpts := sel("Hard", "Hard", "Medium", "Medium", "Easy", "Easy")
	modeOpts := sel("PvP", "PvP", "PvE", "PvE")
	fields := make([]model.ConfigFieldDef, 0, 7)
	for _, g := range openMUGroupCatalog {
		fields = append(fields,
			model.ConfigFieldDef{Key: g.nameKey, Label: "Nome — " + g.name, Type: "text", Placeholder: g.name, Section: section, Hint: "Cabeçalho do grupo no seletor do launcher (Servidor …). Vazio = padrão."},
			model.ConfigFieldDef{Key: g.diffKey, Label: "Dificuldade — " + g.name, Type: "select", Options: diffOpts, Placeholder: g.difficulty, Section: section, Hint: "Tag Hard / Medium / Easy ao lado do nome do servidor."},
		)
	}
	fields = append(fields, model.ConfigFieldDef{
		Key: openMUChannel21ModeKey, Label: "Modo do Canal 2 (99d, ServerID 21)", Type: "select",
		Options: modeOpts, Placeholder: "PvE", Section: section,
		Hint: "Badge PvP/PvE do canal extra de 99d. Os demais canais são PvP.",
	})
	return fields
}

// ResolveOpenMUServerGroups returns the launcher catalog, overlaying any
// operator overrides stored in the OpenMU game-template config fields.
func ResolveOpenMUServerGroups(fields map[string]string) []OpenMUServerGroup {
	out := make([]OpenMUServerGroup, 0, len(openMUGroupCatalog))
	for _, g := range openMUGroupCatalog {
		name, difficulty := g.name, g.difficulty
		if v := strings.TrimSpace(fields[g.nameKey]); v != "" {
			name = v
		}
		if v := strings.TrimSpace(fields[g.diffKey]); v != "" {
			difficulty = v
		}
		channels := append([]OpenMUChannelMeta(nil), g.channels...)
		for i, ch := range channels {
			if ch.ID == 21 {
				if v := strings.TrimSpace(fields[openMUChannel21ModeKey]); v != "" {
					channels[i].Mode = v
				}
			}
		}
		out = append(out, OpenMUServerGroup{
			Group:      g.serverID / 20,
			Name:       name,
			Difficulty: difficulty,
			Channels:   channels,
		})
	}
	return out
}
