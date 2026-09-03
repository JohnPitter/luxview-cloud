package service

import "testing"

func TestResolveOpenMUServerGroupsDefaults(t *testing.T) {
	got := ResolveOpenMUServerGroups(nil)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Group != 1 || got[0].Name != "99d" || got[0].Difficulty != "Hard" {
		t.Fatalf("99d group = %+v", got[0])
	}
	if len(got[0].Channels) != 2 || got[0].Channels[0].ID != 20 || got[0].Channels[0].Mode != "PvP" {
		t.Fatalf("99d canal 1 = %+v", got[0].Channels)
	}
	if got[0].Channels[1].ID != 21 || got[0].Channels[1].Mode != "PvE" {
		t.Fatalf("99d canal 2 = %+v", got[0].Channels[1])
	}
	if got[1].Name != "Season 2" || got[1].Difficulty != "Medium" {
		t.Fatalf("s2 = %+v", got[1])
	}
	if got[2].Name != "Season 6 Pt 3" || got[2].Difficulty != "Easy" {
		t.Fatalf("s6 = %+v", got[2])
	}
}

func TestResolveOpenMUServerGroupsOverrides(t *testing.T) {
	got := ResolveOpenMUServerGroups(map[string]string{
		"OPENMU_S20_NAME":       "Lux 99d",
		"OPENMU_S20_DIFFICULTY": "Medium",
		"OPENMU_S21_MODE":       "PvP",
	})
	if got[0].Name != "Lux 99d" || got[0].Difficulty != "Medium" {
		t.Fatalf("override group = %+v", got[0])
	}
	if got[0].Channels[1].Mode != "PvP" {
		t.Fatalf("mode override = %q", got[0].Channels[1].Mode)
	}
}

func TestOpenMUTemplateIncludesChannelCatalogFields(t *testing.T) {
	tmpl := Template("openmu")
	if tmpl == nil {
		t.Fatal("openmu template missing")
	}
	want := map[string]bool{
		"OPENMU_S0_NAME": true, "OPENMU_S0_DIFFICULTY": true,
		"OPENMU_S20_NAME": true, "OPENMU_S20_DIFFICULTY": true,
		"OPENMU_S40_NAME": true, "OPENMU_S40_DIFFICULTY": true,
		"OPENMU_S21_MODE": true,
	}
	for _, f := range tmpl.ConfigFields {
		delete(want, f.Key)
	}
	if len(want) > 0 {
		t.Fatalf("missing catalog fields: %v", want)
	}
}
