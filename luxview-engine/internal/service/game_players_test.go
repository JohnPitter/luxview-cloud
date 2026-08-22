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
