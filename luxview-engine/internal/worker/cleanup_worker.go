package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// CleanupWorker periodically removes old images, builds, and stale metrics.
type CleanupWorker struct {
	docker     *dockerclient.Client
	metricRepo *repository.MetricRepo
	interval   time.Duration
}

func NewCleanupWorker(docker *dockerclient.Client, metricRepo *repository.MetricRepo, intervalSec int) *CleanupWorker {
	return &CleanupWorker{
		docker:     docker,
		metricRepo: metricRepo,
		interval:   time.Duration(intervalSec) * time.Second,
	}
}

// Start begins the cleanup loop.
func (cw *CleanupWorker) Start(ctx context.Context) {
	log := logger.With("cleanup-worker")
	log.Info().Dur("interval", cw.interval).Msg("starting cleanup worker")

	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("cleanup worker stopped")
			return
		case <-ticker.C:
			cw.cleanup(ctx)
		}
	}
}

func (cw *CleanupWorker) cleanup(ctx context.Context) {
	log := logger.With("cleanup-worker")

	// Remove metrics older than 30 days
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	deleted, err := cw.metricRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("failed to cleanup old metrics")
	} else if deleted > 0 {
		log.Info().Int64("deleted", deleted).Msg("old metrics cleaned up")
	}

	// Prune dangling Docker images
	containers, err := cw.docker.ListContainers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list containers for cleanup")
		return
	}

	activeImages := make(map[string]bool)
	for _, c := range containers {
		activeImages[c.Image] = true
	}

	log.Debug().Int("active_images", len(activeImages)).Msg("cleanup pass completed")
}
