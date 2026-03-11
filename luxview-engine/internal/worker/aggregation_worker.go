package worker

import (
	"context"
	"time"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// AggregationWorker runs daily to compact raw pageviews into aggregations and clean up old data.
type AggregationWorker struct {
	pageviewRepo *repository.PageviewRepo
	interval     time.Duration
}

func NewAggregationWorker(pageviewRepo *repository.PageviewRepo, intervalHours int) *AggregationWorker {
	return &AggregationWorker{
		pageviewRepo: pageviewRepo,
		interval:     time.Duration(intervalHours) * time.Hour,
	}
}

func (aw *AggregationWorker) Start(ctx context.Context) {
	log := logger.With("aggregation-worker")
	log.Info().Dur("interval", aw.interval).Msg("starting aggregation worker")

	// Run once at startup
	aw.run(ctx)

	ticker := time.NewTicker(aw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("aggregation worker stopped")
			return
		case <-ticker.C:
			aw.run(ctx)
		}
	}
}

func (aw *AggregationWorker) run(ctx context.Context) {
	log := logger.With("aggregation-worker")

	// Step 1: Aggregate raw pageviews older than 7 days into hourly buckets
	hourlyThreshold := time.Now().Add(-7 * 24 * time.Hour)
	hourlyRows, err := aw.pageviewRepo.AggregateHourly(ctx, hourlyThreshold)
	if err != nil {
		log.Error().Err(err).Msg("failed to aggregate hourly")
	} else if hourlyRows > 0 {
		log.Info().Int64("rows", hourlyRows).Msg("hourly aggregation completed")
	}

	// Step 2: Compact hourly aggregations older than 30 days into daily
	dailyThreshold := time.Now().Add(-30 * 24 * time.Hour)
	dailyRows, err := aw.pageviewRepo.CompactToDaily(ctx, dailyThreshold)
	if err != nil {
		log.Error().Err(err).Msg("failed to compact to daily")
	} else if dailyRows > 0 {
		log.Info().Int64("rows", dailyRows).Msg("daily compaction completed")
	}

	// Step 3: Purge raw pageviews older than 7 days (already aggregated)
	rawDeleted, err := aw.pageviewRepo.DeleteOlderThan(ctx, hourlyThreshold)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete old raw pageviews")
	} else if rawDeleted > 0 {
		log.Info().Int64("deleted", rawDeleted).Msg("old raw pageviews purged")
	}

	// Step 4: Delete hourly aggregations older than 30 days (already compacted to daily)
	hourlyDeleted, err := aw.pageviewRepo.DeleteHourlyOlderThan(ctx, dailyThreshold)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete old hourly aggregations")
	} else if hourlyDeleted > 0 {
		log.Info().Int64("deleted", hourlyDeleted).Msg("old hourly aggregations purged")
	}

	// Step 5: Delete daily aggregations older than 90 days
	retentionThreshold := time.Now().Add(-90 * 24 * time.Hour)
	dailyDeleted, err := aw.pageviewRepo.DeleteDailyOlderThan(ctx, retentionThreshold)
	if err != nil {
		log.Error().Err(err).Msg("failed to delete old daily aggregations")
	} else if dailyDeleted > 0 {
		log.Info().Int64("deleted", dailyDeleted).Msg("old daily aggregations purged")
	}
}
