package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luxview/engine/internal/api"
	"github.com/luxview/engine/internal/config"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	"github.com/luxview/engine/internal/worker"
	"github.com/luxview/engine/pkg/crypto"
	dockerclient "github.com/luxview/engine/pkg/docker"
	pkggithub "github.com/luxview/engine/pkg/github"
	"github.com/luxview/engine/pkg/logger"
)

func main() {
	// Initialize logger
	logger.Init(os.Getenv("LOG_LEVEL"))
	log := logger.With("main")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	db, err := repository.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Docker client
	docker, err := dockerclient.New()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create docker client")
	}
	defer docker.Close()

	// Encryption key
	encryptionKey := crypto.DeriveKey(cfg.EncryptionKey)

	// Repositories
	userRepo := repository.NewUserRepo(db)
	repositoryRepo := repository.NewRepositoryRepo(db)
	appRepo := repository.NewAppRepo(db)
	deployRepo := repository.NewDeploymentRepo(db)
	actionRepo := repository.NewActionRepo(db, encryptionKey)
	serviceRepo := repository.NewServiceRepo(db)
	metricRepo := repository.NewMetricRepo(db)
	alertRepo := repository.NewAlertRepo(db)
	planRepo := repository.NewPlanRepo(db)
	settingsRepo := repository.NewSettingsRepo(db, encryptionKey)
	auditRepo := repository.NewAuditLogRepo(db)
	auditSvc := service.NewAuditService(auditRepo)
	mailboxRepo := repository.NewMailboxRepo(db)
	backupRepo := repository.NewBackupRepo(db)
	gameConfigRepo := repository.NewGameServerConfigRepo(db)

	// Services
	portManager := service.NewPortManager(appRepo, cfg.PortRangeStart, cfg.PortRangeEnd)
	containerMgr := service.NewContainerManager(docker, cfg.AppNetwork)
	gameServerSvc := service.NewGameServerService(docker, cfg.GameNetwork)
	provisioner := service.NewProvisioner(serviceRepo, mailboxRepo, cfg, encryptionKey)
	routerSvc := service.NewRouterService(appRepo, cfg.Domain)
	repositorySvc := service.NewRepositoryService(repositoryRepo, cfg.RepositoryBasePath)
	// Backup support is wired after githubAppSvc is initialised below.
	sourceCheckout := service.NewAppSourceCheckout(
		service.NewGitHubSourceCheckout(userRepo, encryptionKey, "source-checkout"),
		service.NewLuxViewSourceCheckout(repositorySvc),
	)
	deployer := service.NewDeployer(appRepo, deployRepo, userRepo, serviceRepo, settingsRepo, provisioner, docker, portManager, encryptionKey, sourceCheckout, time.Duration(cfg.BuildTimeout)*time.Second, cfg.AppNetwork)
	metricsCollector := service.NewMetricsCollector(appRepo, metricRepo, docker)
	healthChecker := service.NewHealthChecker(appRepo, containerMgr)
	alerter := service.NewAlerter(alertRepo, metricRepo, appRepo, userRepo, cfg)
	backupSvc := service.NewBackupService(backupRepo, settingsRepo, auditSvc, cfg.BackupDir, service.ContainerConfig{
		PGPlatformContainer: "luxview-pg-platform",
		PGPlatformUser:      "luxview",
		PGSharedContainer:   "luxview-pg-shared",
		PGSharedUser:        cfg.SharedPGUser,
		MongoContainer:      "luxview-mongo-shared",
		MongoUser:           cfg.SharedMongoUser,
		MongoPassword:       cfg.SharedMongoPassword,
		RedisContainer:      "luxview-redis-shared",
		RedisPassword:       cfg.SharedRedisPassword,
	})

	// Workers
	buildWorker, buildQueue := worker.NewBuildWorker(deployer, cfg.BuildConcurrency)
	buildWorker.Start(ctx)

	actionSvc := service.NewActionService(actionRepo, appRepo, sourceCheckout, buildQueue, cfg.ActionArtifactsDir)
	webhookSvc := service.NewWebhookService(appRepo, buildQueue, actionSvc)
	pushEventSvc := service.NewPushEventService(appRepo, repositorySvc, actionSvc, buildQueue)
	pullRequestRepo := repository.NewPullRequestRepo(db)
	pullRequestSvc := service.NewPullRequestService(pullRequestRepo, repositorySvc)

	// GitHub App service (optional — only when GITHUB_APP_ID is set)
	var githubAppSvc *service.GitHubAppService
	if cfg.GitHubAppID != 0 && cfg.GitHubAppPrivateKey != "" {
		appClient, err := pkggithub.NewAppClient(cfg.GitHubAppID, []byte(cfg.GitHubAppPrivateKey))
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create GitHub App client")
		}
		githubAppSvc = service.NewGitHubAppService(appClient, userRepo, encryptionKey)
		log.Info().Int64("app_id", cfg.GitHubAppID).Msg("GitHub App integration enabled")
	} else {
		log.Info().Msg("GitHub App integration disabled (GITHUB_APP_ID not set)")
	}

	// Wire backup support once githubAppSvc is available.
	if githubAppSvc != nil {
		repositorySvc.WithBackupSupport(githubAppSvc, userRepo)
	}

	metricsWorker := worker.NewMetricsWorker(metricsCollector, cfg.MetricsInterval)
	go metricsWorker.Start(ctx)

	healthWorker := worker.NewHealthCheckWorker(healthChecker, cfg.HealthCheckInterval)
	go healthWorker.Start(ctx)

	cleanupWorker := worker.NewCleanupWorker(docker, metricRepo, settingsRepo, auditRepo, cfg.CleanupInterval)
	go cleanupWorker.Start(ctx)

	alertWorker := worker.NewAlertWorker(alerter, cfg.AlertInterval)
	go alertWorker.Start(ctx)

	staleDeployWorker := worker.NewStaleDeployWorker(deployRepo, 60, cfg.BuildTimeout*2)
	go staleDeployWorker.Start(ctx)

	actionWorker := worker.NewActionWorker(actionRepo, actionSvc, cfg.BuildConcurrency)
	actionWorker.Start(ctx)

	// Analytics (GeoIP + log parser + worker)
	geoipSvc := service.NewGeoIP(cfg.GeoLite2Path)
	defer geoipSvc.Close()

	logParser := service.NewLogParser(geoipSvc, cfg.Domain)
	pageviewRepo := repository.NewPageviewRepo(db)

	analyticsWorker := worker.NewAnalyticsWorker(
		cfg.TraefikLogPath,
		logParser,
		pageviewRepo,
		appRepo,
		cfg.AnalyticsInterval,
	)
	go analyticsWorker.Start(ctx)

	aggregationWorker := worker.NewAggregationWorker(pageviewRepo, 24)
	go aggregationWorker.Start(ctx)

	backupWorker := worker.NewBackupWorker(backupSvc, settingsRepo, backupRepo)
	go backupWorker.Start(ctx)

	// Router
	router := api.NewRouter(api.Deps{
		Config:         cfg,
		UserRepo:       userRepo,
		RepositoryRepo: repositoryRepo,
		AppRepo:        appRepo,
		DeployRepo:     deployRepo,
		ActionRepo:     actionRepo,
		ServiceRepo:    serviceRepo,
		MetricRepo:     metricRepo,
		AlertRepo:      alertRepo,
		PlanRepo:       planRepo,
		Container:      containerMgr,
		Provisioner:    provisioner,
		Router:         routerSvc,
		WebhookSvc:     webhookSvc,
		ActionSvc:      actionSvc,
		PushEventSvc:   pushEventSvc,
		RepositorySvc:  repositorySvc,
		GitHubAppSvc:   githubAppSvc,
		BuildQueue:     buildQueue,
		EncryptKey:     encryptionKey,
		SettingsRepo:   settingsRepo,
		Docker:         docker,
		AuditRepo:      auditRepo,
		AuditSvc:       auditSvc,
		PageviewRepo:   pageviewRepo,
		MailboxRepo:    mailboxRepo,
		BackupSvc:       backupSvc,
		PullRequestRepo: pullRequestRepo,
		PullRequestSvc:  pullRequestSvc,
		GameConfigRepo:  gameConfigRepo,
		GameServerSvc:   gameServerSvc,
	})

	// HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")

		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("server shutdown error")
		}

		buildWorker.Stop()
		actionWorker.Stop()
		log.Info().Msg("server stopped gracefully")
	}()

	log.Info().Int("port", cfg.Port).Msg("starting LuxView Engine")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
