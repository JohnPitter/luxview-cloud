package main

import "testing"

func TestMetin2LaunchSpecUsesLegacyClientLayout(t *testing.T) {
	spec, ok := launchSpecs["metin2"]
	if !ok {
		t.Fatal("metin2 launch spec is not registered")
	}
	if spec.clientDir != "Metin2FullClient" || spec.gameExe != "Metin2Distribute.exe" {
		t.Fatalf("legacy client layout = %q/%q", spec.clientDir, spec.gameExe)
	}
}
