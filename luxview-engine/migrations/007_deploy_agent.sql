-- platform_settings: key-value store for admin config (AI keys, etc.)
CREATE TABLE IF NOT EXISTS platform_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    encrypted BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- custom_dockerfile: AI-generated or user-edited Dockerfile stored per app
ALTER TABLE apps ADD COLUMN IF NOT EXISTS custom_dockerfile TEXT DEFAULT NULL;
