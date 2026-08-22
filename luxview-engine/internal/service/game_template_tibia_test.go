package service

import "testing"

func TestTibiaTemplateRates(t *testing.T) {
	tmpl := Template(tibiaTemplateID)
	if tmpl == nil {
		t.Fatal("tibia template not registered")
	}
	want := map[string]string{
		"TIBIA_RATE_EXP":    "Taxas",
		"TIBIA_RATE_SPAWN":  "Taxas",
		"TIBIA_RATE_LOOT":   "Taxas",
		"TIBIA_WORLD_TYPE":  "Servidor",
		"TIBIA_PROTECTION_LEVEL": "Gameplay",
	}
	got := map[string]string{}
	for _, field := range tmpl.ConfigFields {
		got[field.Key] = field.Section
	}
	for key, section := range want {
		if got[key] != section {
			t.Errorf("%s section = %q, want %q", key, got[key], section)
		}
	}
}
