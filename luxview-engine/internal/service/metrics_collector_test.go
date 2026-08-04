package service

import "testing"

func TestEffectiveMemoryUsageSubtractsInactiveFileCache(t *testing.T) {
	got := effectiveMemoryUsage(struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	}{
		Usage: 4_288_323_584,
		Stats: map[string]uint64{"inactive_file": 1_450_258_432},
	})

	want := uint64(2_838_065_152)
	if got != want {
		t.Fatalf("effective memory = %d, want %d", got, want)
	}
}

func TestEffectiveMemoryUsagePrefersTotalInactiveFile(t *testing.T) {
	got := effectiveMemoryUsage(struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	}{
		Usage: 1_000,
		Stats: map[string]uint64{
			"total_inactive_file": 200,
			"inactive_file":       500,
		},
	})

	if got != 800 {
		t.Fatalf("effective memory = %d, want 800", got)
	}
}

func TestEffectiveMemoryUsageNeverBecomesNegative(t *testing.T) {
	got := effectiveMemoryUsage(struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	}{
		Usage: 100,
		Stats: map[string]uint64{"inactive_file": 200},
	})

	if got != 0 {
		t.Fatalf("effective memory = %d, want 0", got)
	}
}
