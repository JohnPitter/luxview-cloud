-- =============================================================================
-- Migration 005: Create metrics table
-- =============================================================================
-- Time-series metrics collected from Docker stats every 30s.
-- Partitioned by month for efficient querying and retention management.
-- =============================================================================

CREATE TABLE metrics (
    id              BIGSERIAL,
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    cpu_percent     DOUBLE PRECISION,
    memory_bytes    BIGINT,
    network_rx      BIGINT,
    network_tx      BIGINT,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Composite index for metric queries: app + time range (on each partition)
CREATE INDEX idx_metrics_app_timestamp ON metrics (app_id, timestamp DESC);

-- Create initial partitions (current month + next 3 months)
-- Engine should auto-create future partitions via a cron/worker.
DO $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
BEGIN
    FOR i IN 0..3 LOOP
        start_date := date_trunc('month', CURRENT_DATE + (i || ' months')::interval);
        end_date := start_date + '1 month'::interval;
        partition_name := 'metrics_' || to_char(start_date, 'YYYY_MM');

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF metrics
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END LOOP;
END $$;
