package model

import "testing"

func TestBackupStatusConstants(t *testing.T) {
	tests := []struct {
		status BackupStatus
		want   string
	}{
		{BackupStatusRunning, "running"},
		{BackupStatusCompleted, "completed"},
		{BackupStatusFailed, "failed"},
		{BackupStatusRestoring, "restoring"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("BackupStatus = %q, want %q", tt.status, tt.want)
		}
	}
}

func TestBackupTriggerConstants(t *testing.T) {
	tests := []struct {
		trigger BackupTrigger
		want    string
	}{
		{BackupTriggerScheduled, "scheduled"},
		{BackupTriggerManual, "manual"},
	}
	for _, tt := range tests {
		if string(tt.trigger) != tt.want {
			t.Errorf("BackupTrigger = %q, want %q", tt.trigger, tt.want)
		}
	}
}

func TestBackupDatabaseConstants(t *testing.T) {
	tests := []struct {
		db   BackupDatabase
		want string
	}{
		{DBPGPlatform, "pg-platform"},
		{DBPGShared, "pg-shared"},
		{DBMongoShared, "mongo-shared"},
		{DBRedisShared, "redis-shared"},
	}
	for _, tt := range tests {
		if string(tt.db) != tt.want {
			t.Errorf("BackupDatabase = %q, want %q", tt.db, tt.want)
		}
	}
}

func TestIsValidDatabase(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"pg-platform", true},
		{"pg-shared", true},
		{"mongo-shared", true},
		{"redis-shared", true},
		{"mysql", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidDatabase(tt.input); got != tt.valid {
			t.Errorf("IsValidDatabase(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidSchedule(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"daily", true},
		{"weekly", true},
		{"monthly", true},
		{"hourly", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidSchedule(tt.input); got != tt.valid {
			t.Errorf("IsValidSchedule(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidRetention(t *testing.T) {
	tests := []struct {
		input int
		valid bool
	}{
		{7, true},
		{14, true},
		{30, true},
		{60, true},
		{0, false},
		{15, false},
		{-1, false},
	}
	for _, tt := range tests {
		if got := IsValidRetention(tt.input); got != tt.valid {
			t.Errorf("IsValidRetention(%d) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}
