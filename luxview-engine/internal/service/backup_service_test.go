package service

import (
	"testing"
	"time"

	"github.com/luxview/engine/internal/model"
)

func TestBuildBackupDirName(t *testing.T) {
	ts := time.Date(2026, 4, 2, 3, 0, 0, 0, time.UTC)

	got := buildBackupDirName(ts, model.BackupTriggerScheduled)
	want := "2026-04-02_030000_scheduled"
	if got != want {
		t.Errorf("buildBackupDirName() = %q, want %q", got, want)
	}

	got = buildBackupDirName(ts, model.BackupTriggerManual)
	want = "2026-04-02_030000_manual"
	if got != want {
		t.Errorf("buildBackupDirName() = %q, want %q", got, want)
	}
}

func TestDumpCommand(t *testing.T) {
	tests := []struct {
		db      string
		wantBin string
	}{
		{"pg-platform", "docker"},
		{"pg-shared", "docker"},
		{"mongo-shared", "docker"},
		{"redis-shared", "docker"},
	}
	for _, tt := range tests {
		cmd := dumpCommand(tt.db, "/backups/test", testContainerConfig())
		if cmd == nil {
			t.Fatalf("dumpCommand(%q) returned nil", tt.db)
		}
		if cmd.Path == "" {
			t.Errorf("dumpCommand(%q) has empty Path", tt.db)
		}
	}
}

func TestRestoreCommand(t *testing.T) {
	tests := []struct {
		db      string
		wantNil bool
	}{
		{"pg-platform", false},
		{"pg-shared", false},
		{"mongo-shared", false},
		{"redis-shared", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		cmd := restoreCommand(tt.db, "/backups/test", testContainerConfig())
		if (cmd == nil) != tt.wantNil {
			t.Errorf("restoreCommand(%q) nil=%v, want nil=%v", tt.db, cmd == nil, tt.wantNil)
		}
	}
}

func TestParseBackupSettings(t *testing.T) {
	m := map[string]string{
		"enabled":        "true",
		"schedule":       "daily",
		"retention_days": "30",
		"databases":      "pg-platform,pg-shared",
	}
	s := ParseBackupSettings(m)
	if !s.Enabled {
		t.Error("expected Enabled=true")
	}
	if s.Schedule != "daily" {
		t.Errorf("Schedule = %q, want daily", s.Schedule)
	}
	if s.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", s.RetentionDays)
	}
	if len(s.Databases) != 2 {
		t.Errorf("Databases len = %d, want 2", len(s.Databases))
	}
}

func TestParseBackupSettingsDefaults(t *testing.T) {
	s := ParseBackupSettings(map[string]string{})
	if s.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if s.Schedule != "daily" {
		t.Errorf("Schedule default = %q, want daily", s.Schedule)
	}
	if s.RetentionDays != 30 {
		t.Errorf("RetentionDays default = %d, want 30", s.RetentionDays)
	}
}

func TestShouldRunNow(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	// daily at 03:00 — exact match
	ts := time.Date(2026, 4, 2, 3, 0, 30, 0, loc)
	if !shouldRunNow("daily", ts, loc) {
		t.Error("daily should run at 03:00")
	}

	// daily at 04:00 — no match
	ts = time.Date(2026, 4, 2, 4, 0, 30, 0, loc)
	if shouldRunNow("daily", ts, loc) {
		t.Error("daily should NOT run at 04:00")
	}

	// weekly on Sunday at 03:00
	// 2026-04-05 is a Sunday
	ts = time.Date(2026, 4, 5, 3, 0, 30, 0, loc)
	if !shouldRunNow("weekly", ts, loc) {
		t.Error("weekly should run on Sunday 03:00")
	}

	// weekly on Monday — no match
	ts = time.Date(2026, 4, 6, 3, 0, 30, 0, loc)
	if shouldRunNow("weekly", ts, loc) {
		t.Error("weekly should NOT run on Monday")
	}

	// monthly on 1st at 03:00
	ts = time.Date(2026, 4, 1, 3, 0, 30, 0, loc)
	if !shouldRunNow("monthly", ts, loc) {
		t.Error("monthly should run on 1st 03:00")
	}

	// monthly on 2nd — no match
	ts = time.Date(2026, 4, 2, 3, 0, 30, 0, loc)
	if shouldRunNow("monthly", ts, loc) {
		t.Error("monthly should NOT run on 2nd")
	}
}

// testContainerConfig returns config for command construction tests.
func testContainerConfig() ContainerConfig {
	return ContainerConfig{
		PGPlatformContainer: "luxview-pg-platform",
		PGPlatformUser:      "luxview",
		PGSharedContainer:   "luxview-pg-shared",
		PGSharedUser:        "luxview_admin",
		MongoContainer:      "luxview-mongo-shared",
		MongoUser:           "luxview_admin",
		MongoPassword:       "testpass",
		RedisContainer:      "luxview-redis-shared",
		RedisPassword:       "testpass",
	}
}
