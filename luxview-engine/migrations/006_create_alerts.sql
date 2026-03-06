-- =============================================================================
-- Migration 006: Create alerts table
-- =============================================================================
-- User-defined alert rules evaluated by the alert worker every 60s.
-- =============================================================================

CREATE TYPE alert_channel AS ENUM ('email', 'webhook', 'discord');

CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    metric          VARCHAR(50) NOT NULL,           -- 'cpu_percent', 'memory_bytes', etc.
    condition       VARCHAR(20) NOT NULL,           -- 'gt', 'lt', 'gte', 'lte', 'eq'
    threshold       DOUBLE PRECISION NOT NULL,
    channel         alert_channel NOT NULL,
    channel_config  JSONB NOT NULL DEFAULT '{}'::jsonb,  -- {email, url, webhook_url}
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_triggered  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup alerts by app
CREATE INDEX idx_alerts_app_id ON alerts (app_id);

-- Only evaluate enabled alerts
CREATE INDEX idx_alerts_enabled ON alerts (app_id) WHERE enabled = true;
