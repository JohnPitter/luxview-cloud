-- =============================================================================
-- Migration 002: Create apps table
-- =============================================================================
-- Each app represents a deployed GitHub repository.
-- env_vars is encrypted at rest with AES-256-GCM.
-- =============================================================================

CREATE TYPE app_status AS ENUM ('building', 'running', 'stopped', 'error', 'sleeping');
CREATE TYPE app_stack AS ENUM ('node', 'python', 'go', 'rust', 'static', 'docker');

CREATE TABLE apps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    subdomain       VARCHAR(100) NOT NULL UNIQUE,  -- global unique slug
    repo_url        TEXT NOT NULL,
    repo_branch     VARCHAR(100) NOT NULL DEFAULT 'main',
    stack           app_stack,
    status          app_status NOT NULL DEFAULT 'stopped',
    container_id    VARCHAR(100),
    internal_port   INT,
    assigned_port   INT UNIQUE,
    env_vars        JSONB,                          -- encrypted AES-256-GCM
    resource_limits JSONB NOT NULL DEFAULT '{"cpu": "0.5", "memory": "512m", "disk": "1g"}'::jsonb,
    auto_deploy     BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- List apps by user
CREATE INDEX idx_apps_user_id ON apps (user_id);

-- Subdomain lookup for Traefik routing
CREATE INDEX idx_apps_subdomain ON apps (subdomain);

-- Port collision avoidance
CREATE INDEX idx_apps_assigned_port ON apps (assigned_port) WHERE assigned_port IS NOT NULL;

-- Constraint: subdomain must be lowercase alphanumeric + hyphens
ALTER TABLE apps ADD CONSTRAINT chk_subdomain_format
    CHECK (subdomain ~ '^[a-z0-9]([a-z0-9-]{0,98}[a-z0-9])?$');

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_apps_updated_at
    BEFORE UPDATE ON apps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
