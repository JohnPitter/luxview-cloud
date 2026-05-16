-- =============================================================================
-- Migration 009: Create actions tables
-- =============================================================================
-- Persistent CI/action runs, jobs and steps for LuxView Actions.
-- =============================================================================

CREATE TABLE action_runs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    workflow      VARCHAR(120) NOT NULL,
    workflow_path TEXT NOT NULL,
    trigger       VARCHAR(40) NOT NULL DEFAULT 'manual',
    branch        VARCHAR(100) NOT NULL DEFAULT '',
    commit_sha    VARCHAR(40) NOT NULL DEFAULT '',
    status        VARCHAR(20) NOT NULL DEFAULT 'queued',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);

CREATE TABLE action_jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id      UUID NOT NULL REFERENCES action_runs(id) ON DELETE CASCADE,
    name        VARCHAR(120) NOT NULL,
    image       VARCHAR(255) NOT NULL DEFAULT 'node:22-alpine',
    status      VARCHAR(20) NOT NULL DEFAULT 'queued',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE action_steps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES action_jobs(id) ON DELETE CASCADE,
    name        VARCHAR(200) NOT NULL,
    kind        VARCHAR(20) NOT NULL,
    command     TEXT NOT NULL DEFAULT '',
    uses        VARCHAR(255) NOT NULL DEFAULT '',
    inputs      JSONB NOT NULL DEFAULT '{}',
    status      VARCHAR(20) NOT NULL DEFAULT 'queued',
    exit_code   INT NOT NULL DEFAULT 0,
    log         TEXT NOT NULL DEFAULT '',
    position    INT NOT NULL,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE action_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key             VARCHAR(100) NOT NULL,
    encrypted_value TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(app_id, key)
);

CREATE TABLE action_artifacts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id     UUID NOT NULL REFERENCES action_runs(id) ON DELETE CASCADE,
    name       VARCHAR(120) NOT NULL,
    path       TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, name)
);

CREATE INDEX idx_action_runs_app_created ON action_runs(app_id, created_at DESC);
CREATE INDEX idx_action_runs_status ON action_runs(status);
CREATE INDEX idx_action_jobs_run ON action_jobs(run_id);
CREATE INDEX idx_action_jobs_status ON action_jobs(status);
CREATE INDEX idx_action_steps_job_position ON action_steps(job_id, position);
CREATE INDEX idx_action_secrets_app ON action_secrets(app_id, key);
CREATE INDEX idx_action_artifacts_run ON action_artifacts(run_id, name);
