package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetin2LaunchSpecUsesLegacyClientLayout(t *testing.T) {
	spec, ok := launchSpecs["metin2"]
	if !ok {
		t.Fatal("metin2 launch spec is not registered")
	}
	if spec.clientDir != "Metin2FullClient" || spec.gameExe != "Metin2Distribute.exe" {
		t.Fatalf("legacy client layout = %q/%q", spec.clientDir, spec.gameExe)
	}
	if spec.loginPath != "" || spec.registerPath != "" {
		t.Fatalf("Metin2 must authenticate inside the game, got login=%q register=%q", spec.loginPath, spec.registerPath)
	}
	if spec.settingsINI != "metin2.cfg" {
		t.Fatalf("Metin2 settings file = %q, want metin2.cfg", spec.settingsINI)
	}
}

func TestTibiaLaunchSpecUsesOTClient(t *testing.T) {
	spec, ok := launchSpecForGame("Tibia (Canary)")
	if !ok {
		t.Fatal("tibia launch spec is not registered")
	}
	if spec.clientDir != "" || spec.gameExe != "otclient.exe" {
		t.Fatalf("Tibia client layout = %q/%q", spec.clientDir, spec.gameExe)
	}
}

func TestNormalizeGameIDAcceptsTibiaVariants(t *testing.T) {
	for _, raw := range []string{"tibia", "Tibia", "tibia-canary", "Tibia (Canary)"} {
		if got := normalizeGameID(raw); got != "tibia" {
			t.Fatalf("normalizeGameID(%q) = %q, want tibia", raw, got)
		}
	}
}

func TestResolveGameIDFallsBackToTibiaDisplayName(t *testing.T) {
	got := resolveGameID(GameCard{DisplayName: "Tibia (Canary)"})
	if got != "tibia" {
		t.Fatalf("resolveGameID() = %q, want tibia", got)
	}
}

func TestClientNeedsUpdateWhenCatalogHashDiffers(t *testing.T) {
	if clientNeedsUpdate(false, "abc", "") {
		t.Fatal("uninstalled games should not ask for a client update")
	}
	if clientNeedsUpdate(true, "", "abc") {
		t.Fatal("empty catalog hash should not force an update")
	}
	if clientNeedsUpdate(true, "abc", "abc") {
		t.Fatal("matching hashes should be up to date")
	}
	if !clientNeedsUpdate(true, "new", "") {
		t.Fatal("legacy installs without a stamp should update once")
	}
	if !clientNeedsUpdate(true, "new", "old") {
		t.Fatal("replaced client zip should mark the install outdated")
	}
}

func TestMetin2InstallRequiresRuntimeFiles(t *testing.T) {
	spec := launchSpecs["metin2"]
	root := t.TempDir()
	clientDir := filepath.Join(root, spec.clientDir)
	for _, file := range requiredClientFiles("metin2", spec) {
		path := filepath.Join(clientDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("client"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !clientFilesReady(root, "metin2", spec) {
		t.Fatal("complete Metin2 client was not recognized")
	}
	if err := os.Remove(filepath.Join(clientDir, "python27.dll")); err != nil {
		t.Fatal(err)
	}
	if clientFilesReady(root, "metin2", spec) {
		t.Fatal("Metin2 client without python27.dll was recognized as ready")
	}
}

func TestMetin2SettingsRoundTrip(t *testing.T) {
	content := "WIDTH\t1366\nHEIGHT\t708\nMUSIC_VOLUME\t0.361\nWINDOWED\t1\nSHADOW_LEVEL\t3\n"
	settings := parseMetin2Settings(content, defaultMetin2Settings())
	if settings.ScreenWidth != 1366 || settings.ScreenHeight != 708 || !settings.Windowed {
		t.Fatalf("parsed settings = %+v", settings)
	}

	settings.ScreenWidth = 1920
	settings.MusicVolume = 0.8
	settings.Windowed = false
	settings.ShadowLevel = 1
	updated := writeMetin2Config(content, settings)
	parsed := parseMetin2Settings(updated, defaultMetin2Settings())
	if parsed.ScreenWidth != 1920 || parsed.MusicVolume != 0.8 || parsed.Windowed || parsed.ShadowLevel != 1 {
		t.Fatalf("round-trip settings = %+v", parsed)
	}
}
