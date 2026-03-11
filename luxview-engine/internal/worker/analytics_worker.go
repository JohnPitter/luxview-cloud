package worker

import (
	"bufio"
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/pkg/logger"
)

const maxBatchSize = 1000

// AnalyticsWorker reads Traefik access logs and inserts pageviews.
type AnalyticsWorker struct {
	logPath      string
	parser       *service.LogParser
	pageviewRepo *repository.PageviewRepo
	appRepo      *repository.AppRepo
	interval     time.Duration
	offset       int64
}

func NewAnalyticsWorker(
	logPath string,
	parser *service.LogParser,
	pageviewRepo *repository.PageviewRepo,
	appRepo *repository.AppRepo,
	intervalSec int,
) *AnalyticsWorker {
	return &AnalyticsWorker{
		logPath:      logPath,
		parser:       parser,
		pageviewRepo: pageviewRepo,
		appRepo:      appRepo,
		interval:     time.Duration(intervalSec) * time.Second,
	}
}

func (aw *AnalyticsWorker) Start(ctx context.Context) {
	log := logger.With("analytics-worker")

	if aw.logPath == "" {
		log.Warn().Msg("no TRAEFIK_LOG_PATH configured, analytics worker disabled")
		return
	}

	log.Info().Str("log_path", aw.logPath).Dur("interval", aw.interval).Msg("starting analytics worker")

	// Seek to end of file on startup (don't process historical logs)
	if info, err := os.Stat(aw.logPath); err == nil {
		aw.offset = info.Size()
		log.Info().Int64("initial_offset", aw.offset).Msg("skipping existing log lines")
	}

	ticker := time.NewTicker(aw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("analytics worker stopped")
			return
		case <-ticker.C:
			aw.collect(ctx)
		}
	}
}

func (aw *AnalyticsWorker) collect(ctx context.Context) {
	log := logger.With("analytics-worker")

	// Refresh app cache
	apps, err := aw.appRepo.ListAllSubdomains(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list apps for analytics cache")
		return
	}

	appMap := make(map[string]uuid.UUID, len(apps))
	for _, app := range apps {
		appMap[app.Subdomain] = app.ID
	}
	aw.parser.UpdateAppCache(appMap)

	// Open log file
	f, err := os.Open(aw.logPath)
	if err != nil {
		log.Debug().Err(err).Msg("cannot open traefik log file")
		return
	}
	defer f.Close()

	// Handle file rotation (if file is smaller than offset, it was rotated)
	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < aw.offset {
		aw.offset = 0
		log.Info().Msg("log file rotated, resetting offset")
	}

	// Seek to last position
	if _, err := f.Seek(aw.offset, io.SeekStart); err != nil {
		log.Error().Err(err).Msg("failed to seek log file")
		return
	}

	// Read new lines
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var batch []model.Pageview
	linesRead := 0
	pageviewsCollected := 0

	for scanner.Scan() {
		linesRead++
		pv := aw.parser.ParseLine(scanner.Bytes())
		if pv == nil {
			continue
		}
		batch = append(batch, *pv)
		pageviewsCollected++

		if len(batch) >= maxBatchSize {
			if err := aw.pageviewRepo.InsertBatch(ctx, batch); err != nil {
				log.Error().Err(err).Int("count", len(batch)).Msg("failed to insert pageview batch")
			}
			batch = batch[:0]
		}
	}

	// Insert remaining
	if len(batch) > 0 {
		if err := aw.pageviewRepo.InsertBatch(ctx, batch); err != nil {
			log.Error().Err(err).Int("count", len(batch)).Msg("failed to insert pageview batch")
		}
	}

	// Update offset
	newOffset, _ := f.Seek(0, io.SeekCurrent)
	aw.offset = newOffset

	if linesRead > 0 {
		log.Debug().Int("lines", linesRead).Int("pageviews", pageviewsCollected).Msg("analytics batch processed")
	}
}
