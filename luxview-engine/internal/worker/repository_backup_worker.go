package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

type RepositoryBackupWorker struct {
	repositoryRepo *repository.RepositoryRepo
	repositorySvc  *service.RepositoryService
	interval       time.Duration
}

func NewRepositoryBackupWorker(repositoryRepo *repository.RepositoryRepo, repositorySvc *service.RepositoryService) *RepositoryBackupWorker {
	return &RepositoryBackupWorker{
		repositoryRepo: repositoryRepo,
		repositorySvc:  repositorySvc,
		interval:       time.Minute,
	}
}

func (w *RepositoryBackupWorker) Start(ctx context.Context) {
	log := logger.With("repository-backup-worker")
	log.Info().Msg("starting repository backup retry worker")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("repository backup retry worker stopped")
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

func (w *RepositoryBackupWorker) processPending(ctx context.Context) {
	log := logger.With("repository-backup-worker")
	targets, err := w.repositoryRepo.ListPendingBackupTargets(ctx, 100)
	if err != nil {
		log.Warn().Err(err).Msg("failed to list pending repository backups")
		return
	}
	for _, target := range targets {
		if err := w.repositorySvc.SyncBackup(ctx, target.RepositoryID, target.RemoteID, target.UserID); err != nil {
			log.Warn().Err(err).Str("repository_id", target.RepositoryID.String()).Str("remote_id", target.RemoteID.String()).Msg("repository backup retry failed")
		}
	}
}
