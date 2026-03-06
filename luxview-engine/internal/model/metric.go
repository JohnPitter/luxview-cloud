package model

import (
	"time"

	"github.com/google/uuid"
)

type Metric struct {
	ID          int64     `json:"id"`
	AppID       uuid.UUID `json:"app_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryBytes int64     `json:"memory_bytes"`
	NetworkRx   int64     `json:"network_rx"`
	NetworkTx   int64     `json:"network_tx"`
	Timestamp   time.Time `json:"timestamp"`
}

type MetricAggregation struct {
	Timestamp   time.Time `json:"timestamp"`
	AvgCPU      float64   `json:"avg_cpu"`
	MaxCPU      float64   `json:"max_cpu"`
	AvgMemory   float64   `json:"avg_memory"`
	MaxMemory   int64     `json:"max_memory"`
	AvgNetRx    float64   `json:"avg_network_rx"`
	AvgNetTx    float64   `json:"avg_network_tx"`
}
