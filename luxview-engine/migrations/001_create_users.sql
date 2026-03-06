-- =============================================================================
-- Migration 001: Create users table
-- =============================================================================
-- Users authenticated via GitHub OAuth.
-- github_token is encrypted at rest with AES-256-GCM.
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE user_role AS ENUM ('user', 'admin');

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id       BIGINT NOT NULL UNIQUE,
    username        VARCHAR(100) NOT NULL,
    email           VARCHAR(255),
    avatar_url      TEXT,
    github_token    TEXT,                          -- encrypted AES-256-GCM
    role            user_role NOT NULL DEFAULT 'user',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);

-- Fast lookup by GitHub ID during OAuth callback
CREATE INDEX idx_users_github_id ON users (github_id);
