package worker

import "testing"

func TestNewBackupWorker(t *testing.T) {
	w := NewBackupWorker(nil, nil, nil)
	if w == nil {
		t.Fatal("NewBackupWorker returned nil")
	}
	if w.checkInterval != 60 {
		t.Errorf("checkInterval = %d, want 60", w.checkInterval)
	}
}
