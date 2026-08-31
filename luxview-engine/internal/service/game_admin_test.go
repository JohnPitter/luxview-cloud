package service

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestAdminPanelURLOpenMU(t *testing.T) {
	cfg := &model.GameServerConfig{
		TemplateID: "openmu",
		ExtraPorts: []model.ExtraPort{
			{Port: 55980, Protocol: "tcp", Label: "ChatServer"},
			{Port: 18080, Protocol: "tcp", Label: "Painel Admin (localhost:18080 — SSH tunnel)"},
		},
	}
	got, ok := AdminPanelURL("mu", cfg)
	if !ok || got != "http://luxview-game-mu:18080" {
		t.Fatalf("AdminPanelURL = %q %v", got, ok)
	}
}

func TestAdminPanelURLOpenMUWithoutExtraPorts(t *testing.T) {
	cfg := &model.GameServerConfig{TemplateID: "openmu"}
	got, ok := AdminPanelURL("mu", cfg)
	if !ok || got != "http://luxview-game-mu:18080" {
		t.Fatalf("fallback AdminPanelURL = %q %v", got, ok)
	}
}

func TestAdminPanelURLOtherGames(t *testing.T) {
	cfg := &model.GameServerConfig{
		TemplateID: "tibia",
		ExtraPorts: []model.ExtraPort{{Port: 8088, Protocol: "tcp", Label: "Login HTTP"}},
	}
	if _, ok := AdminPanelURL("tibia1", cfg); ok {
		t.Fatal("tibia login HTTP is not an admin panel")
	}
}
