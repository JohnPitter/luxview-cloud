package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/luxview/engine/internal/api/handlers"
	"github.com/luxview/engine/internal/api/middleware"
	"github.com/luxview/engine/internal/config"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/internal/service"
	dockerclient "github.com/luxview/engine/pkg/docker"
)

// Deps holds all dependencies needed to set up the router.
type Deps struct {
	Config      *config.Config
	UserRepo    *repository.UserRepo
	AppRepo     *repository.AppRepo
	DeployRepo  *repository.DeploymentRepo
	ServiceRepo *repository.ServiceRepo
	MetricRepo  *repository.MetricRepo
	AlertRepo   *repository.AlertRepo
	Container   *service.ContainerManager
	Provisioner *service.Provisioner
	Router      *service.RouterService
	WebhookSvc  *service.WebhookService
	BuildQueue  chan<- service.DeployRequest
	EncryptKey  []byte
	PlanRepo     *repository.PlanRepo
	SettingsRepo *repository.SettingsRepo
	Docker       *dockerclient.Client
	AuditRepo    *repository.AuditLogRepo
	AuditSvc     *service.AuditService
	PageviewRepo *repository.PageviewRepo
	MailboxRepo  *repository.MailboxRepo
	BackupSvc *service.BackupService
}

// NewRouter creates the main HTTP router with all routes.
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173", "https://luxview.cloud", "https://dashboard.luxview.cloud"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestLogger)

	// Request body size limit: 1MB (CWE-770)
	r.Use(middleware.BodySizeLimit(1 << 20))

	// Rate limiter: 20 req/s with burst of 40
	rl := middleware.NewRateLimiter(20, 40)
	r.Use(rl.Middleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(deps.Config, deps.UserRepo, deps.SettingsRepo, deps.EncryptKey, deps.AuditSvc)
	webhookURL := deps.Config.BaseURL + "/api/webhooks/github"
	appHandler := handlers.NewAppHandler(deps.AppRepo, deps.UserRepo, deps.ServiceRepo, deps.Container, deps.Provisioner, deps.BuildQueue, deps.EncryptKey, deps.AuditSvc, webhookURL, deps.Config.InternalToken)
	deployHandler := handlers.NewDeploymentHandler(deps.DeployRepo, deps.AppRepo, deps.BuildQueue, deps.AuditSvc)
	serviceHandler := handlers.NewServiceHandler(deps.ServiceRepo, deps.AppRepo, deps.Provisioner, deps.EncryptKey, deps.AuditSvc)
	metricHandler := handlers.NewMetricHandler(deps.MetricRepo, deps.AppRepo)
	alertHandler := handlers.NewAlertHandler(deps.AlertRepo, deps.AppRepo, deps.AuditSvc)
	adminHandler := handlers.NewAdminHandler(deps.UserRepo, deps.AppRepo, deps.DeployRepo, deps.ServiceRepo, deps.Container, deps.Provisioner, deps.AuditSvc)
	explorerHandler := handlers.NewExplorerHandler(deps.ServiceRepo, deps.AppRepo, deps.EncryptKey)
	traefikHandler := handlers.NewTraefikHandler(deps.Router)
	webhookHandler := handlers.NewWebhookHandler(deps.WebhookSvc, deps.Config.InternalToken)
	planHandler := handlers.NewPlanHandler(deps.PlanRepo, deps.UserRepo, deps.AppRepo, deps.AuditSvc)
	settingsHandler := handlers.NewSettingsHandler(deps.SettingsRepo, deps.AuditSvc)
	analyzeHandler := handlers.NewAnalyzeHandler(deps.AppRepo, deps.UserRepo, deps.DeployRepo, deps.SettingsRepo, deps.ServiceRepo, deps.Provisioner, deps.EncryptKey, deps.AuditSvc)
	cleanupHandler := handlers.NewCleanupHandler(deps.SettingsRepo, deps.Docker, deps.AuditSvc)
	auditHandler := handlers.NewAuditHandler(deps.AuditRepo)
	analyticsHandler := handlers.NewAnalyticsHandler(deps.PageviewRepo, deps.AppRepo)
	mailboxHandler := handlers.NewMailboxHandler(deps.MailboxRepo, deps.ServiceRepo, deps.AppRepo, deps.Provisioner, deps.AuditSvc, deps.Config.Domain)
	backupHandler := handlers.NewBackupHandler(deps.BackupSvc, deps.AuditSvc)

	authMiddleware := middleware.Auth(deps.Config.JWTSecret, deps.UserRepo)

	r.Route("/api", func(r chi.Router) {
		// Health check (public, for status page)
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		})

		// Auth (public, stricter rate limit: 3 req/s burst 6)
		r.Group(func(r chi.Router) {
			authRL := middleware.NewRateLimiter(3, 6)
			r.Use(authRL.Middleware)
			r.Get("/auth/github", authHandler.GitHubLogin)
			r.Get("/auth/github/callback", authHandler.GitHubCallback)
		})

		// Plans (public, for landing page)
		r.Get("/plans", planHandler.ListActive)

		// Auth settings (public, for dashboard public mode check)
		r.Get("/auth/settings", settingsHandler.GetAuthSettings)

		// Webhooks (public, verified by signature)
		r.Post("/webhooks/github", webhookHandler.GitHubWebhook)

		// Internal (Traefik)
		r.Group(func(r chi.Router) {
			r.Use(middleware.InternalAuth(deps.Config.InternalToken))
			r.Get("/internal/traefik-config", traefikHandler.GetConfig)
		})

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			// User
			r.Get("/auth/me", authHandler.Me)

			// GitHub
			r.Get("/github/repos", appHandler.ListGitHubRepos)
			r.Get("/github/repos/{owner}/{repo}/branches", appHandler.ListGitHubBranches)

			// Apps
			r.Get("/apps/check-subdomain/{subdomain}", appHandler.CheckSubdomain)
			r.Post("/apps", appHandler.Create)
			r.Get("/apps", appHandler.List)
			r.Get("/apps/{id}", appHandler.Get)
			r.Patch("/apps/{id}", appHandler.Update)
			r.Delete("/apps/{id}", appHandler.Delete)
			r.Post("/apps/{id}/deploy", appHandler.Deploy)
			r.Post("/apps/{id}/restart", appHandler.Restart)
			r.Post("/apps/{id}/stop", appHandler.Stop)
			r.Put("/apps/{id}/maintenance", appHandler.SetMaintenance)
			r.Get("/apps/{id}/logs", appHandler.ContainerLogs)
			r.Get("/apps/{id}/logs/stream", appHandler.ContainerLogsStream)

			// AI Analyze
			r.Post("/apps/{id}/analyze", analyzeHandler.Analyze)
			r.Post("/apps/{id}/analyze-failure", analyzeHandler.AnalyzeFailure)
			r.Put("/apps/{id}/dockerfile", analyzeHandler.SaveDockerfile)
			r.Delete("/apps/{id}/dockerfile", analyzeHandler.DeleteDockerfile)
			r.Post("/apps/{id}/apply-analysis", analyzeHandler.ApplyAnalysis)

			// Deployments
			r.Get("/deployments/recent", deployHandler.ListRecent)
			r.Get("/apps/{id}/deployments", deployHandler.List)
			r.Get("/deployments/{id}/logs", deployHandler.GetLogs)
			r.Post("/deployments/{id}/rollback", deployHandler.Rollback)

			// Services
			r.Get("/services", serviceHandler.ListAll)
			r.Post("/apps/{id}/services", serviceHandler.Create)
			r.Get("/apps/{id}/services", serviceHandler.List)
			r.Delete("/services/{id}", serviceHandler.Delete)

			// Mailboxes
			r.Get("/services/{id}/mailboxes", mailboxHandler.List)
			r.Post("/services/{id}/mailboxes", mailboxHandler.Create)
			r.Delete("/mailboxes/{id}", mailboxHandler.Delete)
			r.Post("/mailboxes/{id}/reset-password", mailboxHandler.ResetPassword)

			// Explorer (DB + Storage)
			r.Get("/services/{id}/tables", explorerHandler.ListTables)
			r.Get("/services/{id}/tables/{table}", explorerHandler.GetTableSchema)
			r.Post("/services/{id}/query", explorerHandler.ExecuteQuery)
			r.Get("/services/{id}/files", explorerHandler.ListFiles)
			r.Post("/services/{id}/files/upload", explorerHandler.UploadFile)
			r.Get("/services/{id}/files/download", explorerHandler.DownloadFile)
			r.Delete("/services/{id}/files", explorerHandler.DeleteFile)
			r.Get("/services/{id}/usage", explorerHandler.ServiceUsage)

			// Metrics
			r.Get("/apps/metrics/latest", metricHandler.LatestAll)
			r.Get("/apps/{id}/metrics", metricHandler.Get)

			// Alerts
			r.Post("/apps/{id}/alerts", alertHandler.Create)
			r.Get("/apps/{id}/alerts", alertHandler.List)
			r.Patch("/alerts/{id}", alertHandler.Update)
			r.Delete("/alerts/{id}", alertHandler.Delete)

			// Analytics
			r.Get("/analytics/overview", analyticsHandler.Overview)
			r.Get("/analytics/pages", analyticsHandler.Pages)
			r.Get("/analytics/geo", analyticsHandler.Geo)
			r.Get("/analytics/browsers", analyticsHandler.Browsers)
			r.Get("/analytics/os", analyticsHandler.OS)
			r.Get("/analytics/devices", analyticsHandler.Devices)
			r.Get("/analytics/referers", analyticsHandler.Referers)
			r.Get("/analytics/live", analyticsHandler.Live)

			// Admin (5 req/s burst 10)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AdminOnly)
				adminRL := middleware.NewRateLimiter(5, 10)
				r.Use(adminRL.Middleware)
				r.Get("/admin/users", adminHandler.ListUsers)
				r.Get("/admin/stats", adminHandler.Stats)
				r.Delete("/admin/apps/{id}", adminHandler.ForceDeleteApp)
				r.Get("/admin/apps", adminHandler.ListAllApps)
				r.Patch("/admin/users/{id}/role", adminHandler.UpdateUserRole)
				r.Patch("/admin/apps/{id}/limits", adminHandler.UpdateAppLimits)
				r.Get("/admin/vps-info", adminHandler.VPSInfo)
				r.Get("/admin/plans", planHandler.ListAll)
				r.Post("/admin/plans", planHandler.Create)
				r.Patch("/admin/plans/{id}", planHandler.Update)
				r.Delete("/admin/plans/{id}", planHandler.Delete)
				r.Patch("/admin/plans/{id}/default", planHandler.SetDefault)
				r.Patch("/admin/users/{id}/plan", planHandler.AssignUserPlan)
				r.Get("/admin/settings/ai", settingsHandler.GetAISettings)
				r.Put("/admin/settings/ai", settingsHandler.UpdateAISettings)
				r.Post("/admin/settings/ai/test", settingsHandler.TestAIConnection)
				r.Get("/admin/settings/timezone", settingsHandler.GetTimezone)
				r.Put("/admin/settings/timezone", settingsHandler.UpdateTimezone)
				r.Get("/admin/settings/cleanup", cleanupHandler.GetCleanupSettings)
				r.Put("/admin/settings/cleanup", cleanupHandler.UpdateCleanupSettings)
				r.Post("/admin/cleanup/trigger", cleanupHandler.TriggerCleanup)
				r.Get("/admin/cleanup/disk-usage", cleanupHandler.DiskUsage)
				r.Put("/admin/settings/auth", settingsHandler.UpdateAuthSettings)
				r.Get("/admin/audit-logs", auditHandler.ListAuditLogs)
				r.Get("/admin/audit-logs/stats", auditHandler.AuditStats)
				r.Get("/admin/backups", backupHandler.List)
				r.Post("/admin/backups", backupHandler.Trigger)
				r.Get("/admin/backups/settings", backupHandler.GetSettings)
				r.Put("/admin/backups/settings", backupHandler.UpdateSettings)
				r.Get("/admin/backups/{id}", backupHandler.Get)
				r.Delete("/admin/backups/{id}", backupHandler.Delete)
				r.Post("/admin/backups/{id}/restore", backupHandler.Restore)
				r.Get("/admin/backups/{id}/download", backupHandler.Download)
			})
		})
	})

	return r
}
