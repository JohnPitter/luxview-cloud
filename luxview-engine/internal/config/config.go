package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port    int
	BaseURL string
	Domain  string

	DatabaseURL string

	EncryptionKey string
	JWTSecret     string

	GitHubClientID     string
	GitHubClientSecret string

	SharedPGHost     string
	SharedPGPort     int
	SharedPGUser     string
	SharedPGPassword string

	SharedRedisHost     string
	SharedRedisPort     int
	SharedRedisPassword string

	SharedMongoHost     string
	SharedMongoPort     int
	SharedMongoUser     string
	SharedMongoPassword string

	SharedRabbitHost     string
	SharedRabbitPort     int
	SharedRabbitUser     string
	SharedRabbitPassword string

	SharedMinioHost     string
	SharedMinioPort     int
	SharedMinioUser     string
	SharedMinioPassword string

	StorageBasePath string // base path for local storage volumes

	BuildConcurrency int
	PortRangeStart   int
	PortRangeEnd     int

	MetricsInterval     int // seconds
	HealthCheckInterval int // seconds
	CleanupInterval     int // seconds
	AlertInterval       int // seconds

	BuildTimeout int // seconds

	AppNetwork    string // Docker network for user app containers
	InternalToken string

	// Analytics
	GeoLite2Path      string
	TraefikLogPath    string
	AnalyticsInterval int // seconds
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:   envInt("PORT", 8080),
		Domain: envStr("DOMAIN", "luxview.cloud"),

		DatabaseURL: envStr("DATABASE_URL", "postgres://luxview:luxview@localhost:5432/luxview_platform?sslmode=disable"),

		EncryptionKey: envStr("ENCRYPTION_KEY", ""),
		JWTSecret:     envStr("JWT_SECRET", ""),

		GitHubClientID:     envStr("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: envStr("GITHUB_CLIENT_SECRET", ""),

		SharedPGHost:     envStr("SHARED_PG_HOST", "pg-shared"),
		SharedPGPort:     envInt("SHARED_PG_PORT", 5432),
		SharedPGUser:     envStr("SHARED_PG_USER", "luxview_admin"),
		SharedPGPassword: envStr("SHARED_PG_PASSWORD", ""),

		SharedRedisHost:     envStr("SHARED_REDIS_HOST", "redis-shared"),
		SharedRedisPort:     envInt("SHARED_REDIS_PORT", 6379),
		SharedRedisPassword: envStr("SHARED_REDIS_PASSWORD", ""),

		SharedMongoHost:     envStr("SHARED_MONGO_HOST", "mongo-shared"),
		SharedMongoPort:     envInt("SHARED_MONGO_PORT", 27017),
		SharedMongoUser:     envStr("SHARED_MONGO_USER", "luxview_admin"),
		SharedMongoPassword: envStr("SHARED_MONGO_PASSWORD", ""),

		SharedRabbitHost:     envStr("SHARED_RABBIT_HOST", "rabbitmq-shared"),
		SharedRabbitPort:     envInt("SHARED_RABBIT_PORT", 5672),
		SharedRabbitUser:     envStr("SHARED_RABBIT_USER", "luxview_admin"),
		SharedRabbitPassword: envStr("SHARED_RABBIT_PASSWORD", ""),

		SharedMinioHost:     envStr("SHARED_MINIO_HOST", "minio-shared"),
		SharedMinioPort:     envInt("SHARED_MINIO_PORT", 9000),
		SharedMinioUser:     envStr("SHARED_MINIO_USER", "luxview_admin"),
		SharedMinioPassword: envStr("SHARED_MINIO_PASSWORD", ""),

		StorageBasePath: envStr("STORAGE_BASE_PATH", "/data/luxview/storage"),

		BuildConcurrency: envInt("BUILD_CONCURRENCY", 3),
		PortRangeStart:   envInt("PORT_RANGE_START", 10000),
		PortRangeEnd:     envInt("PORT_RANGE_END", 65000),

		MetricsInterval:     envInt("METRICS_INTERVAL", 30),
		HealthCheckInterval: envInt("HEALTHCHECK_INTERVAL", 15),
		CleanupInterval:     envInt("CLEANUP_INTERVAL", 3600),
		AlertInterval:       envInt("ALERT_INTERVAL", 60),

		BuildTimeout: envInt("BUILD_TIMEOUT", 300),

		AppNetwork:    envStr("APP_NETWORK", "luxview-net"),
		InternalToken: envStr("INTERNAL_TOKEN", ""),

		GeoLite2Path:      envStr("GEOLITE2_PATH", "/usr/share/GeoIP/GeoLite2-City.mmdb"),
		TraefikLogPath:    envStr("TRAEFIK_LOG_PATH", ""),
		AnalyticsInterval: envInt("ANALYTICS_INTERVAL", 60),
	}

	// Derive BaseURL from DOMAIN if not explicitly set
	cfg.BaseURL = envStr("BASE_URL", fmt.Sprintf("https://%s", cfg.Domain))

	var missing []string
	if cfg.EncryptionKey == "" {
		missing = append(missing, "ENCRYPTION_KEY")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.GitHubClientID == "" {
		missing = append(missing, "GITHUB_CLIENT_ID")
	}
	if cfg.GitHubClientSecret == "" {
		missing = append(missing, "GITHUB_CLIENT_SECRET")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
