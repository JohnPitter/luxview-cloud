package service

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestSkipPublishedPort(t *testing.T) {
	if !skipPublishedPort("tibia", 3306) {
		t.Fatal("mysql must stay unpublished")
	}
	if !skipPublishedPort("tibia", 8080) {
		t.Fatal("tibia 8080 must stay unpublished")
	}
	if skipPublishedPort("tibia", 8088) {
		t.Fatal("login HTTP must stay published")
	}
}

func TestSkipMetinExtraCores(t *testing.T) {
	cfg := &model.GameServerConfig{
		TemplateID:   "metin2",
		ConfigFields: map[string]string{"METIN_CORE_COUNT": "1"},
	}
	if !skipMetinExtra(cfg, 13002) {
		t.Fatal("core 2 should stay closed when METIN_CORE_COUNT=1")
	}
	if !skipMetinExtra(cfg, 13099) {
		t.Fatal("game99 should stay closed with a single core")
	}
	cfg.ConfigFields["METIN_CORE_COUNT"] = "4"
	if skipMetinExtra(cfg, 13004) {
		t.Fatal("core 4 should publish when METIN_CORE_COUNT=4")
	}
}
