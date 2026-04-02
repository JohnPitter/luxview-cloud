package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type BackupWorker struct {
	backupSvc     *service.BackupService
	settingsRepo  *repository.SettingsRepo
	backupRepo    *repository.BackupRepo
	checkInterval int // seconds between checks (60)
	lastRunDate   string
}

func NewBackupWorker(backupSvc *service.BackupService, settingsRepo *repository.SettingsRepo, backupRepo *repository.BackupRepo) *BackupWorker {
	return &BackupWorker{
		backupSvc:     backupSvc,
		settingsRepo:  settingsRepo,
		backupRepo:    backupRepo,
		checkInterval: 60,
	}
}

func (w *BackupWorker) Start(ctx context.Context) {
	log := logger.With("backup-worker")
	log.Info().Msg("starting backup scheduler")

	ticker := time.NewTicker(time.Duration(w.checkInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("backup scheduler stopped")
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *BackupWorker) check(ctx context.Context) {
	log := logger.With("backup-worker")

	settings := w.backupSvc.GetSettings(ctx)
	if !settings.Enabled || len(settings.Databases) == 0 {
		return
	}

	tz, _ := w.settingsRepo.Get(ctx, "platform_timezone")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	dateKey := now.Format("2006-01-02-15-04") + "-" + settings.Schedule
	if dateKey == w.lastRunDate {
		return
	}

	if now.Hour() != 3 || now.Minute() != 0 {
		return
	}

	shouldRun := false
	switch settings.Schedule {
	case "daily":
		shouldRun = true
	case "weekly":
		shouldRun = now.Weekday() == time.Sunday
	case "monthly":
		shouldRun = now.Day() == 1
	}

	if !shouldRun {
		return
	}

	w.lastRunDate = dateKey
	log.Info().Str("schedule", settings.Schedule).Strs("databases", settings.Databases).Msg("scheduled backup triggered")

	_, err = w.backupSvc.Run(ctx, settings.Databases, model.BackupTriggerScheduled, nil)
	if err != nil {
		log.Error().Err(err).Msg("scheduled backup failed")
	}

	if err := w.backupSvc.Cleanup(ctx, settings.RetentionDays); err != nil {
		log.Error().Err(err).Msg("backup cleanup failed")
	}
}
