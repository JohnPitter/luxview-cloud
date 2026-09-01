package service

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestTibiaVocationAndTown(t *testing.T) {
	if tibiaVocationName(8) != "Elite Knight" {
		t.Fatalf("vocation 8 = %q", tibiaVocationName(8))
	}
	if tibiaTownName(8) != "Thais" {
		t.Fatalf("town 8 = %q", tibiaTownName(8))
	}
}

func TestMetinJobAndMap(t *testing.T) {
	if metinJobName(5) != "Ninja (F)" {
		t.Fatalf("job 5 = %q", metinJobName(5))
	}
	if metinJobName(8) != "Lycan" {
		t.Fatalf("job 8 = %q", metinJobName(8))
	}
	if metinMapName(1) != "Chunjo — Vila" {
		t.Fatalf("map 1 = %q", metinMapName(1))
	}
}

func TestMuClassAndMap(t *testing.T) {
	if muClassName(17) != "Blade Knight" {
		t.Fatalf("class 17 = %q", muClassName(17))
	}
	if muMapName(0) != "Lorencia" {
		t.Fatalf("map 0 = %q", muMapName(0))
	}
}

func TestParseRoster(t *testing.T) {
	out := "KnightOne\t42\t8\t8:32369,32241,7\n"
	got := parseRoster(out, func(cols []string) model.PlayerInfo {
		return model.PlayerInfo{Name: cols[0], Character: cols[0]}
	})
	if len(got) != 1 || got[0].Name != "KnightOne" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseOpenMUPlayersJSON(t *testing.T) {
	body := `{"players":[{"character":"CapSparrow","account":"caps","class":"Dark Knight","level":120,"location":"Lorencia","server_id":20,"server_description":"LuxMu (99d)"}]}`
	got, err := parseOpenMUPlayersJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Character != "CapSparrow" || got[0].Server != "Season 99d - 1" {
		t.Fatalf("got %+v", got[0])
	}
}

func TestFormatOpenMUServer(t *testing.T) {
	cases := []struct {
		id   int
		desc string
		want string
	}{
		{0, "LuxMu (S6)", "Season 6 - 1"},
		{20, "LuxMu (99d)", "Season 99d - 1"},
		{40, "LuxMu (S2)", "Season 2 - 1"},
		{1, "LuxMu (S6)", "Season 6 - 2"},
		{0, "", ""},
	}
	for _, c := range cases {
		got := formatOpenMUServer(c.id, c.desc)
		if got != c.want {
			t.Fatalf("id=%d desc=%q got %q want %q", c.id, c.desc, got, c.want)
		}
	}
}

func TestMapOpenMUHTTPPlayer(t *testing.T) {
	got := mapOpenMUHTTPPlayer(struct {
		Character         string `json:"character"`
		Account           string `json:"account"`
		Class             string `json:"class"`
		Level             int    `json:"level"`
		Location          string `json:"location"`
		ServerID          int    `json:"server_id"`
		ServerDescription string `json:"server_description"`
	}{
		Character: "testgmDK", Class: "Blade Master", Level: 400, Location: "Lorencia",
		ServerID: 0, ServerDescription: "LuxMu (S6)",
	})
	if got.Server != "Season 6 - 1" {
		t.Fatalf("server = %q", got.Server)
	}
}
