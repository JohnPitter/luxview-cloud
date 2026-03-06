-- =============================================================================
-- Migration 003: Create deployments table
-- =============================================================================
-- Deployment history for each app. Tracks build status, logs, and duration.
-- =============================================================================

CREATE TYPE deployment_status AS ENUM (
    'pending', 'building', 'deploying', 'live', 'failed', 'rolled_back'
);

CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    commit_sha      VARCHAR(40),
    commit_message  TEXT,
    status          deployment_status NOT NULL DEFAULT 'pending',
    build_log       TEXT,
    duration_ms     INT,
    image_tag       VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

-- Deployment history per app, most recent first
CREATE INDEX idx_deployments_app_id_created ON deployments (app_id, created_at DESC);
