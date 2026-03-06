-- =============================================================================
-- Migration 004: Create app_services table
-- =============================================================================
-- Provisioned shared services (DB, Redis, etc.) per app.
-- credentials is encrypted at rest with AES-256-GCM.
-- =============================================================================

CREATE TYPE service_type AS ENUM ('postgres', 'redis', 'mongodb', 'rabbitmq');

CREATE TABLE app_services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    service_type    service_type NOT NULL,
    db_name         VARCHAR(100) NOT NULL,
    credentials     JSONB,                          -- encrypted AES-256-GCM
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One service of each type per app
    UNIQUE (app_id, service_type)
);

-- Lookup services by app
CREATE INDEX idx_app_services_app_id ON app_services (app_id);
