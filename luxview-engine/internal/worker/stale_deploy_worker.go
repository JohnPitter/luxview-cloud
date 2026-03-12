package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// StaleDeployWorker periodically checks for deployments stuck in
// pending/building and marks them as failed after a configurable timeout.
type StaleDeployWorker struct {
	deployRepo *repository.DeploymentRepo
	interval   time.Duration
	maxAgeSec  int
}

func NewStaleDeployWorker(deployRepo *repository.DeploymentRepo, intervalSec, maxAgeSec int) *StaleDeployWorker {
	return &StaleDeployWorker{
		deployRepo: deployRepo,
		interval:   time.Duration(intervalSec) * time.Second,
		maxAgeSec:  maxAgeSec,
	}
}

func (w *StaleDeployWorker) Start(ctx context.Context) {
	log := logger.With("stale-deploy-worker")
	log.Info().
		Dur("interval", w.interval).
		Int("max_age_sec", w.maxAgeSec).
		Msg("starting stale deploy worker")

	// Run once at startup to catch anything stuck before engine restarted.
	w.sweep(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("stale deploy worker stopped")
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *StaleDeployWorker) sweep(ctx context.Context) {
	log := logger.With("stale-deploy-worker")
	affected, err := w.deployRepo.FailStale(ctx, w.maxAgeSec)
	if err != nil {
		log.Error().Err(err).Msg("failed to sweep stale deployments")
		return
	}
	if affected > 0 {
		log.Warn().Int64("affected", affected).Msg("marked stale deployments as failed")
	}
}
