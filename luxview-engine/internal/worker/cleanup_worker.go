package worker

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// CleanupWorker periodically removes old images, builds, and stale metrics.
type CleanupWorker struct {
	docker       *dockerclient.Client
	metricRepo   *repository.MetricRepo
	settingsRepo *repository.SettingsRepo
	auditRepo    *repository.AuditLogRepo
	interval     time.Duration
}

func NewCleanupWorker(docker *dockerclient.Client, metricRepo *repository.MetricRepo, settingsRepo *repository.SettingsRepo, auditRepo *repository.AuditLogRepo, intervalSec int) *CleanupWorker {
	return &CleanupWorker{
		docker:       docker,
		metricRepo:   metricRepo,
		settingsRepo: settingsRepo,
		auditRepo:    auditRepo,
		interval:     time.Duration(intervalSec) * time.Second,
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

	// Remove audit logs older than 90 days
	auditCutoff := time.Now().Add(-90 * 24 * time.Hour)
	auditDeleted, err := cw.auditRepo.DeleteOlderThan(ctx, auditCutoff)
	if err != nil {
		log.Error().Err(err).Msg("failed to cleanup old audit logs")
	} else if auditDeleted > 0 {
		log.Info().Int64("deleted", auditDeleted).Msg("old audit logs cleaned up")
	}

	// Check if Docker cleanup is enabled
	settings, err := cw.settingsRepo.GetAll(ctx, "cleanup_")
	if err != nil {
		log.Debug().Err(err).Msg("failed to read cleanup settings, skipping docker prune")
		return
	}

	if settings["enabled"] != "true" {
		return
	}

	// Check disk threshold
	thresholdStr := settings["threshold_percent"]
	if thresholdStr == "" {
		thresholdStr = "80"
	}
	threshold, _ := strconv.Atoi(thresholdStr)
	if threshold <= 0 {
		threshold = 80
	}

	diskPercent := readDiskPercent()
	if diskPercent > 0 && diskPercent < threshold {
		log.Debug().Int("disk_percent", diskPercent).Int("threshold", threshold).Msg("disk usage below threshold, skipping prune")
		return
	}

	log.Info().Int("disk_percent", diskPercent).Int("threshold", threshold).Msg("disk usage above threshold, running docker prune")
	result, err := cw.docker.SystemPrune(ctx)
	if err != nil {
		log.Error().Err(err).Msg("docker system prune failed")
		return
	}

	log.Info().
		Int("images_removed", result.ImagesRemoved).
		Int("containers_removed", result.ContainersRemoved).
		Int64("total_reclaimed_bytes", result.TotalReclaimed).
		Msg("docker system prune completed")
}

// readDiskPercent reads the current root disk usage percentage via df.
func readDiskPercent() int {
	out, err := exec.Command("df", "--output=pcent", "/").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0
	}
	pct := strings.TrimSpace(lines[1])
	pct = strings.TrimSuffix(pct, "%")
	val, _ := strconv.Atoi(pct)
	return val
}
