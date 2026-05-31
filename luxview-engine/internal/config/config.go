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

	// GitHub App (replaces OAuth for new installations)
	GitHubAppID            int64
	GitHubAppSlug          string // e.g. "luxview-cloud"
	GitHubAppClientID      string
	GitHubAppClientSecret  string
	GitHubAppPrivateKey    string // PEM-encoded RSA private key
	GitHubAppWebhookSecret string

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

	StorageBasePath    string // base path for local storage volumes
	RepositoryBasePath string // base path for LuxView-hosted Git repositories
	MailContainerName  string // docker-mailserver container name
	BackupDir          string // base directory for backup files
	ActionArtifactsDir string // base directory for action artifacts

	BuildConcurrency int
	PortRangeStart   int
	PortRangeEnd     int

	MetricsInterval     int // seconds
	HealthCheckInterval int // seconds
	CleanupInterval     int // seconds
	AlertInterval       int // seconds

	BuildTimeout int // seconds

	AppNetwork    string // Docker network for user app containers
	GameNetwork   string // Docker network for game server containers
	InternalToken string

	VPSPublicIP   string // Public IP A-record users must point custom domains to
	AcmeStorePath string // Path to Traefik acme.json (read-only mount), used for cert status

	OpenMUClientBaseZipPath string // Base Season 6 client zip used to generate configured downloads
	RakionClientBaseZipPath string // Base Rakion client zip used to generate configured downloads

	LauncherReleaseRepo string // GitHub "owner/name" whose Releases publish the launcher .exe
	LauncherAssetName   string // Release asset filename for the launcher (e.g. luxview-launcher.exe)
	GitHubAPIToken      string // optional token to raise the GitHub API rate limit for release lookups

	// SMTP (alert emails)
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

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

		GitHubAppID:            envInt64("GITHUB_APP_ID", 0),
		GitHubAppSlug:          envStr("GITHUB_APP_SLUG", ""),
		GitHubAppClientID:      envStr("GITHUB_APP_CLIENT_ID", ""),
		GitHubAppClientSecret:  envStr("GITHUB_APP_CLIENT_SECRET", ""),
		GitHubAppPrivateKey:    envStr("GITHUB_APP_PRIVATE_KEY", ""),
		GitHubAppWebhookSecret: envStr("GITHUB_APP_WEBHOOK_SECRET", ""),

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

		StorageBasePath:    envStr("STORAGE_BASE_PATH", "/data/luxview/storage"),
		RepositoryBasePath: envStr("REPOSITORY_BASE_PATH", "/data/luxview/repositories"),
		MailContainerName:  envStr("MAIL_CONTAINER_NAME", "luxview-mailserver"),
		BackupDir:          envStr("BACKUP_DIR", "/backups"),
		ActionArtifactsDir: envStr("ACTION_ARTIFACTS_DIR", "/data/luxview/action-artifacts"),

		BuildConcurrency: envInt("BUILD_CONCURRENCY", 3),
		PortRangeStart:   envInt("PORT_RANGE_START", 10000),
		PortRangeEnd:     envInt("PORT_RANGE_END", 65000),

		MetricsInterval:     envInt("METRICS_INTERVAL", 30),
		HealthCheckInterval: envInt("HEALTHCHECK_INTERVAL", 15),
		CleanupInterval:     envInt("CLEANUP_INTERVAL", 3600),
		AlertInterval:       envInt("ALERT_INTERVAL", 60),

		BuildTimeout: envInt("BUILD_TIMEOUT", 300),

		AppNetwork:    envStr("APP_NETWORK", "luxview-net"),
		GameNetwork:   envStr("GAME_NETWORK", "game-net"),
		InternalToken: envStr("INTERNAL_TOKEN", ""),

		VPSPublicIP:   envStr("VPS_PUBLIC_IP", ""),
		AcmeStorePath: envStr("ACME_STORE_PATH", "/letsencrypt/acme.json"),

		OpenMUClientBaseZipPath: envStr("OPENMU_CLIENT_BASE_ZIP", "/opt/luxview/openmu-assets/openmu-s6-base.zip"),
		RakionClientBaseZipPath: envStr("RAKION_CLIENT_BASE_ZIP", "/opt/luxview/rakion-assets/rakion-client-base.zip"),

		LauncherReleaseRepo: envStr("LAUNCHER_RELEASE_REPO", "JohnPitter/luxview-cloud"),
		LauncherAssetName:   envStr("LAUNCHER_ASSET_NAME", "luxview-launcher.exe"),
		GitHubAPIToken:      envStr("GITHUB_TOKEN", ""),

		SMTPHost:     envStr("SMTP_HOST", ""),
		SMTPPort:     envInt("SMTP_PORT", 587),
		SMTPUser:     envStr("SMTP_USER", ""),
		SMTPPassword: envStr("SMTP_PASSWORD", ""),
		SMTPFrom:     envStr("SMTP_FROM", "alerts@luxview.cloud"),

		GeoLite2Path:      envStr("GEOLITE2_PATH", "/usr/share/GeoIP/GeoLite2-City.mmdb"),
		TraefikLogPath:    envStr("TRAEFIK_LOG_PATH", ""),
		AnalyticsInterval: envInt("ANALYTICS_INTERVAL", 60),
	}

	// Derive BaseURL from DOMAIN if not explicitly set
	cfg.BaseURL = envStr("BASE_URL", fmt.Sprintf("https://%s", cfg.Domain))

	var missing []string
	if cfg.EncryptionKey == "" {
		missing = append(missing, "ENCRYPTION_KEY")
	} else if len(cfg.EncryptionKey) < 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be at least 32 characters (got %d)", len(cfg.EncryptionKey))
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	} else if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(cfg.JWTSecret))
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

func envInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}
