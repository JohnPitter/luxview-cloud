package service

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// MetricsCollector collects container stats and stores them as metrics.
type MetricsCollector struct {
	appRepo    *repository.AppRepo
	metricRepo *repository.MetricRepo
	docker     *dockerclient.Client
}

func NewMetricsCollector(appRepo *repository.AppRepo, metricRepo *repository.MetricRepo, docker *dockerclient.Client) *MetricsCollector {
	return &MetricsCollector{
		appRepo:    appRepo,
		metricRepo: metricRepo,
		docker:     docker,
	}
}

// CollectAll gathers stats from all running containers and stores them.
func (mc *MetricsCollector) CollectAll(ctx context.Context) {
	log := logger.With("metrics")

	apps, err := mc.appRepo.ListAllRunning(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list running apps")
		return
	}

	var metrics []model.Metric
	for _, app := range apps {
		if app.ContainerID == "" {
			continue
		}

		m, err := mc.collectOne(ctx, &app)
		if err != nil {
			log.Debug().Err(err).Str("app", app.Subdomain).Msg("failed to collect stats")
			continue
		}
		metrics = append(metrics, *m)
	}

	if len(metrics) > 0 {
		if err := mc.metricRepo.InsertBatch(ctx, metrics); err != nil {
			log.Error().Err(err).Msg("failed to insert metrics batch")
		} else {
			log.Debug().Int("count", len(metrics)).Msg("metrics collected")
		}
	}
}

func (mc *MetricsCollector) collectOne(ctx context.Context, app *model.App) (*model.Metric, error) {
	statsResp, err := mc.docker.ContainerStats(ctx, app.ContainerID)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	data, err := io.ReadAll(statsResp.Body)
	if err != nil {
		return nil, err
	}

	var stats dockerStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	cpuPercent := calculateCPUPercent(&stats)
	memoryBytes := stats.MemoryStats.Usage

	var netRx, netTx int64
	for _, netStats := range stats.Networks {
		netRx += netStats.RxBytes
		netTx += netStats.TxBytes
	}

	return &model.Metric{
		AppID:       app.ID,
		CPUPercent:  cpuPercent,
		MemoryBytes: int64(memoryBytes),
		NetworkRx:   netRx,
		NetworkTx:   netTx,
		Timestamp:   time.Now(),
	}, nil
}

func calculateCPUPercent(stats *dockerStats) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)

	if sysDelta > 0 && cpuDelta > 0 {
		numCPUs := float64(stats.CPUStats.OnlineCPUs)
		if numCPUs == 0 {
			numCPUs = 1
		}
		return (cpuDelta / sysDelta) * numCPUs * 100.0
	}
	return 0
}

// dockerStats represents the JSON response from Docker stats API.
type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes int64 `json:"rx_bytes"`
		TxBytes int64 `json:"tx_bytes"`
	} `json:"networks"`
}
