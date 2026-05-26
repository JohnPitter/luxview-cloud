ALTER TABLE apps ADD COLUMN IF NOT EXISTS app_type VARCHAR NOT NULL DEFAULT 'web';

CREATE TABLE IF NOT EXISTS game_server_configs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id      UUID        NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    template_id VARCHAR     NOT NULL,
    image       TEXT        NOT NULL,
    game_port   INTEGER     NOT NULL,
    query_port  INTEGER,
    data_dir    TEXT        NOT NULL DEFAULT '/data',
    data_volume TEXT,
    protocol    VARCHAR     NOT NULL DEFAULT 'udp',
    config_fields JSONB     NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS game_server_configs_app_id_idx ON game_server_configs(app_id);
