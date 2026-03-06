package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type MetricRepo struct {
	db *DB
}

func NewMetricRepo(db *DB) *MetricRepo {
	return &MetricRepo{db: db}
}

func (r *MetricRepo) Insert(ctx context.Context, m *model.Metric) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO metrics (app_id, cpu_percent, memory_bytes, network_rx, network_tx, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		m.AppID, m.CPUPercent, m.MemoryBytes, m.NetworkRx, m.NetworkTx, m.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert metric: %w", err)
	}
	return nil
}

func (r *MetricRepo) InsertBatch(ctx context.Context, metrics []model.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	batch := &batchInsert{}
	for _, m := range metrics {
		batch.add(m)
	}

	_, err := r.db.Pool.Exec(ctx, batch.query(), batch.args()...)
	return err
}

func (r *MetricRepo) GetAggregated(ctx context.Context, appID uuid.UUID, from, to time.Time, intervalSec int) ([]model.MetricAggregation, error) {
	query := fmt.Sprintf(`
		SELECT
			date_trunc('second', timestamp) - (EXTRACT(EPOCH FROM timestamp)::int %% $4) * interval '1 second' AS bucket,
			AVG(cpu_percent) AS avg_cpu,
			MAX(cpu_percent) AS max_cpu,
			AVG(memory_bytes) AS avg_memory,
			MAX(memory_bytes) AS max_memory,
			AVG(network_rx) AS avg_network_rx,
			AVG(network_tx) AS avg_network_tx
		FROM metrics
		WHERE app_id = $1 AND timestamp >= $2 AND timestamp <= $3
		GROUP BY bucket
		ORDER BY bucket ASC`)

	rows, err := r.db.Pool.Query(ctx, query, appID, from, to, intervalSec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.MetricAggregation
	for rows.Next() {
		var a model.MetricAggregation
		if err := rows.Scan(&a.Timestamp, &a.AvgCPU, &a.MaxCPU, &a.AvgMemory, &a.MaxMemory, &a.AvgNetRx, &a.AvgNetTx); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

func (r *MetricRepo) GetLatest(ctx context.Context, appID uuid.UUID) (*model.Metric, error) {
	var m model.Metric
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, app_id, cpu_percent, memory_bytes, network_rx, network_tx, timestamp
		 FROM metrics WHERE app_id = $1 ORDER BY timestamp DESC LIMIT 1`, appID,
	).Scan(&m.ID, &m.AppID, &m.CPUPercent, &m.MemoryBytes, &m.NetworkRx, &m.NetworkTx, &m.Timestamp)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MetricRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM metrics WHERE timestamp < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// batchInsert helper for building multi-row INSERTs.
type batchInsert struct {
	metrics []model.Metric
}

func (b *batchInsert) add(m model.Metric) {
	b.metrics = append(b.metrics, m)
}

func (b *batchInsert) query() string {
	q := `INSERT INTO metrics (app_id, cpu_percent, memory_bytes, network_rx, network_tx, timestamp) VALUES `
	for i := range b.metrics {
		if i > 0 {
			q += ", "
		}
		base := i * 6
		q += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", base+1, base+2, base+3, base+4, base+5, base+6)
	}
	return q
}

func (b *batchInsert) args() []interface{} {
	args := make([]interface{}, 0, len(b.metrics)*6)
	for _, m := range b.metrics {
		args = append(args, m.AppID, m.CPUPercent, m.MemoryBytes, m.NetworkRx, m.NetworkTx, m.Timestamp)
	}
	return args
}
