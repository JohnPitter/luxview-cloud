-- =============================================================================
-- Migration 010: Create internal repositories
-- =============================================================================
-- LuxView-hosted Git repositories are the primary source for new apps.
-- GitHub remotes are optional backup/mirror targets.
-- =============================================================================

CREATE TABLE IF NOT EXISTS repositories (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           VARCHAR(100) NOT NULL,
    slug           VARCHAR(120) NOT NULL,
    default_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    storage_path   TEXT NOT NULL,
    visibility     VARCHAR(20) NOT NULL DEFAULT 'private',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, slug),
    CONSTRAINT chk_repositories_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,118}[a-z0-9])?$'),
    CONSTRAINT chk_repositories_visibility CHECK (visibility IN ('private', 'public'))
);

CREATE INDEX IF NOT EXISTS idx_repositories_user_created ON repositories(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS repository_remotes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id    UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    provider         VARCHAR(40) NOT NULL,
    remote_url       TEXT NOT NULL,
    mode             VARCHAR(20) NOT NULL DEFAULT 'backup',
    last_sync_at     TIMESTAMPTZ,
    last_sync_status VARCHAR(20),
    last_sync_error  TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(repository_id, provider, remote_url),
    CONSTRAINT chk_repository_remotes_mode CHECK (mode IN ('backup', 'mirror')),
    CONSTRAINT chk_repository_remotes_sync_status CHECK (
        last_sync_status IS NULL OR last_sync_status IN ('pending', 'success', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_repository_remotes_repository ON repository_remotes(repository_id);

ALTER TABLE apps ADD COLUMN IF NOT EXISTS repository_id UUID REFERENCES repositories(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_apps_repository_id ON apps(repository_id) WHERE repository_id IS NOT NULL;
