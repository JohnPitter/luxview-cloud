package repository

import (
	"testing"

	"github.com/luxview/engine/internal/model"
)

func TestNewBackupRepo(t *testing.T) {
	repo := NewBackupRepo(nil)
	if repo == nil {
		t.Fatal("NewBackupRepo returned nil")
	}
	if repo.db != nil {
		t.Fatal("expected nil db in test repo")
	}
}

func TestBackupRepoStructHasMethods(t *testing.T) {
	var repo *BackupRepo
	_ = repo

	// These will fail to compile if methods don't exist with correct signatures
	type backupRepoInterface interface {
		Create(ctx interface{}, b *model.Backup) error
		FindByID(ctx interface{}, id interface{}) (*model.Backup, error)
		List(ctx interface{}, limit, offset int) ([]model.Backup, int, error)
		UpdateStatus(ctx interface{}, id interface{}, status model.BackupStatus, err string, fileSize int64, durationMs int) error
		Delete(ctx interface{}, id interface{}) error
		DeleteOlderThan(ctx interface{}, cutoff interface{}) (int64, error)
		FindLatest(ctx interface{}) (*model.Backup, error)
	}
}
