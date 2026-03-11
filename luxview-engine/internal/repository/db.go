package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luxview/engine/pkg/logger"
)

// DB holds the database connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new connection pool and runs migrations.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	log := logger.With("database")

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().Msg("database connection established")

	db := &DB{Pool: pool}
	if err := db.migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) migrate(ctx context.Context) error {
	log := logger.With("migration")

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			github_id BIGINT UNIQUE NOT NULL,
			username VARCHAR(100) NOT NULL,
			email VARCHAR(255) NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			github_token TEXT NOT NULL DEFAULT '',
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ
		)`,

		`CREATE TABLE IF NOT EXISTS apps (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			subdomain VARCHAR(100) UNIQUE NOT NULL,
			repo_url TEXT NOT NULL,
			repo_branch VARCHAR(100) NOT NULL DEFAULT 'main',
			stack VARCHAR(50) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'stopped',
			container_id VARCHAR(100) NOT NULL DEFAULT '',
			internal_port INT NOT NULL DEFAULT 0,
			assigned_port INT UNIQUE,
			env_vars JSONB NOT NULL DEFAULT '{}',
			resource_limits JSONB NOT NULL DEFAULT '{"cpu":"0.5","memory":"512m","disk":"1g"}',
			auto_deploy BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_apps_user_id ON apps(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_apps_subdomain ON apps(subdomain)`,

		`CREATE TABLE IF NOT EXISTS deployments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			commit_sha VARCHAR(40) NOT NULL DEFAULT '',
			commit_message TEXT NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			build_log TEXT NOT NULL DEFAULT '',
			duration_ms INT NOT NULL DEFAULT 0,
			image_tag VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			finished_at TIMESTAMPTZ
		)`,

		`CREATE INDEX IF NOT EXISTS idx_deployments_app_id_created ON deployments(app_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS app_services (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			service_type VARCHAR(20) NOT NULL,
			db_name VARCHAR(100) NOT NULL DEFAULT '',
			credentials JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_app_services_app_id ON app_services(app_id)`,

		`CREATE TABLE IF NOT EXISTS metrics (
			id BIGSERIAL PRIMARY KEY,
			app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
			memory_bytes BIGINT NOT NULL DEFAULT 0,
			network_rx BIGINT NOT NULL DEFAULT 0,
			network_tx BIGINT NOT NULL DEFAULT 0,
			timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_metrics_app_id_ts ON metrics(app_id, timestamp DESC)`,

		`CREATE TABLE IF NOT EXISTS alerts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			metric VARCHAR(50) NOT NULL,
			condition VARCHAR(20) NOT NULL,
			threshold DOUBLE PRECISION NOT NULL,
			channel VARCHAR(20) NOT NULL DEFAULT 'webhook',
			channel_config JSONB NOT NULL DEFAULT '{}',
			enabled BOOLEAN NOT NULL DEFAULT true,
			last_triggered TIMESTAMPTZ
		)`,

		`CREATE INDEX IF NOT EXISTS idx_alerts_app_id ON alerts(app_id)`,

		`CREATE TABLE IF NOT EXISTS plans (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			price DECIMAL(10,2) NOT NULL DEFAULT 0,
			currency VARCHAR(3) NOT NULL DEFAULT 'USD',
			billing_cycle VARCHAR(20) NOT NULL DEFAULT 'monthly',
			max_apps INT NOT NULL DEFAULT 1,
			max_cpu_per_app DECIMAL(4,2) NOT NULL DEFAULT 0.25,
			max_memory_per_app VARCHAR(10) NOT NULL DEFAULT '512m',
			max_disk_per_app VARCHAR(10) NOT NULL DEFAULT '1g',
			max_services_per_app INT NOT NULL DEFAULT 1,
			auto_deploy_enabled BOOLEAN NOT NULL DEFAULT false,
			custom_domain_enabled BOOLEAN NOT NULL DEFAULT false,
			priority_builds BOOLEAN NOT NULL DEFAULT false,
			highlighted BOOLEAN NOT NULL DEFAULT false,
			sort_order INT NOT NULL DEFAULT 0,
			features JSONB NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT true,
			is_default BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`ALTER TABLE users ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id)`,

		`CREATE INDEX IF NOT EXISTS idx_plans_active_order ON plans(is_active, sort_order)`,

		`CREATE TABLE IF NOT EXISTS platform_settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			encrypted BOOLEAN NOT NULL DEFAULT false,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`ALTER TABLE apps ADD COLUMN IF NOT EXISTS custom_dockerfile TEXT DEFAULT NULL`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
			actor_username VARCHAR(100) NOT NULL,
			action VARCHAR(20) NOT NULL,
			resource_type VARCHAR(30) NOT NULL,
			resource_id VARCHAR(100),
			resource_name VARCHAR(200),
			old_values JSONB,
			new_values JSONB,
			ip_address INET,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id)`,

		`ALTER TABLE deployments ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'auto'`,

		// Analytics tables
		`CREATE TABLE IF NOT EXISTS pageviews (
			id          BIGSERIAL PRIMARY KEY,
			app_id      UUID REFERENCES apps(id) ON DELETE CASCADE,
			timestamp   TIMESTAMPTZ NOT NULL,
			path        VARCHAR(2048) NOT NULL,
			method      VARCHAR(10) NOT NULL DEFAULT 'GET',
			status_code SMALLINT NOT NULL,
			ip_hash     VARCHAR(64) NOT NULL,
			country     VARCHAR(2),
			city        VARCHAR(128),
			region      VARCHAR(128),
			browser     VARCHAR(64),
			browser_ver VARCHAR(32),
			os          VARCHAR(64),
			device_type VARCHAR(16),
			referer     VARCHAR(2048),
			response_ms INTEGER
		)`,

		`CREATE INDEX IF NOT EXISTS idx_pv_app_ts ON pageviews(app_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_pv_app_ip ON pageviews(app_id, ip_hash, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_pv_ts ON pageviews(timestamp DESC)`,

		`CREATE TABLE IF NOT EXISTS pageview_aggregations (
			id              BIGSERIAL PRIMARY KEY,
			app_id          UUID REFERENCES apps(id) ON DELETE CASCADE,
			bucket          TIMESTAMPTZ NOT NULL,
			granularity     VARCHAR(4) NOT NULL,
			path            VARCHAR(2048),
			views           INTEGER NOT NULL DEFAULT 0,
			visitors        INTEGER NOT NULL DEFAULT 0,
			bounces         INTEGER NOT NULL DEFAULT 0,
			avg_duration_ms INTEGER,
			country         VARCHAR(2),
			browser         VARCHAR(64),
			os              VARCHAR(64),
			device_type     VARCHAR(16),
			referer_domain  VARCHAR(256)
		)`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_pva_unique ON pageview_aggregations(
			app_id, bucket, granularity,
			COALESCE(path, ''), COALESCE(country, ''), COALESCE(browser, ''),
			COALESCE(os, ''), COALESCE(device_type, ''), COALESCE(referer_domain, ''))`,

		`CREATE INDEX IF NOT EXISTS idx_pva_app_bucket ON pageview_aggregations(app_id, bucket DESC, granularity)`,

		`ALTER TABLE apps ADD COLUMN IF NOT EXISTS webhook_id BIGINT DEFAULT NULL`,
	}

	for i, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	log.Info().Int("count", len(migrations)).Msg("migrations applied")
	return nil
}
