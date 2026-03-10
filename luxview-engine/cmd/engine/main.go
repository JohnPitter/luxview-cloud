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
	appRepo := repository.NewAppRepo(db)
	deployRepo := repository.NewDeploymentRepo(db)
	serviceRepo := repository.NewServiceRepo(db)
	metricRepo := repository.NewMetricRepo(db)
	alertRepo := repository.NewAlertRepo(db)
	planRepo := repository.NewPlanRepo(db)
	settingsRepo := repository.NewSettingsRepo(db, encryptionKey)

	// Services
	portManager := service.NewPortManager(appRepo, cfg.PortRangeStart, cfg.PortRangeEnd)
	containerMgr := service.NewContainerManager(docker, cfg.AppNetwork)
	provisioner := service.NewProvisioner(serviceRepo, cfg, encryptionKey)
	routerSvc := service.NewRouterService(appRepo, cfg.Domain)
	deployer := service.NewDeployer(appRepo, deployRepo, userRepo, serviceRepo, provisioner, docker, portManager, encryptionKey, time.Duration(cfg.BuildTimeout)*time.Second, cfg.AppNetwork)
	metricsCollector := service.NewMetricsCollector(appRepo, metricRepo, docker)
	healthChecker := service.NewHealthChecker(appRepo, containerMgr)
	alerter := service.NewAlerter(alertRepo, metricRepo, appRepo)

	// Workers
	buildWorker, buildQueue := worker.NewBuildWorker(deployer, cfg.BuildConcurrency)
	buildWorker.Start(ctx)

	webhookSvc := service.NewWebhookService(appRepo, buildQueue)

	metricsWorker := worker.NewMetricsWorker(metricsCollector, cfg.MetricsInterval)
	go metricsWorker.Start(ctx)

	healthWorker := worker.NewHealthCheckWorker(healthChecker, cfg.HealthCheckInterval)
	go healthWorker.Start(ctx)

	cleanupWorker := worker.NewCleanupWorker(docker, metricRepo, settingsRepo, cfg.CleanupInterval)
	go cleanupWorker.Start(ctx)

	alertWorker := worker.NewAlertWorker(alerter, cfg.AlertInterval)
	go alertWorker.Start(ctx)

	// Router
	router := api.NewRouter(api.Deps{
		Config:      cfg,
		UserRepo:    userRepo,
		AppRepo:     appRepo,
		DeployRepo:  deployRepo,
		ServiceRepo: serviceRepo,
		MetricRepo:  metricRepo,
		AlertRepo:   alertRepo,
		PlanRepo:    planRepo,
		Container:   containerMgr,
		Provisioner: provisioner,
		Router:      routerSvc,
		WebhookSvc:  webhookSvc,
		BuildQueue:  buildQueue,
		EncryptKey:   encryptionKey,
		SettingsRepo: settingsRepo,
		Docker:       docker,
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
		log.Info().Msg("server stopped gracefully")
	}()

	log.Info().Int("port", cfg.Port).Msg("starting LuxView Engine")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
