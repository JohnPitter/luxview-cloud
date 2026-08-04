package config

import "testing"

func TestLoadUsesRepoOpenMUClientAssetsPathByDefault(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("GITHUB_CLIENT_ID", "client-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "client-secret")
	t.Setenv("OPENMU_CLIENT_BASE_ZIP", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.OpenMUClientBaseZipPath != "/opt/luxview/openmu-assets/openmu-s6-base.zip" {
		t.Fatalf("OpenMUClientBaseZipPath = %q", cfg.OpenMUClientBaseZipPath)
	}
	if cfg.Metin2LegacyClientBaseZipPath != "/opt/luxview/metin2-legacy-assets/metin2-legacy-client-base.zip" {
		t.Fatalf("Metin2LegacyClientBaseZipPath = %q", cfg.Metin2LegacyClientBaseZipPath)
	}
}
