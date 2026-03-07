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

	// Rate limiter: 20 req/s with burst of 40
	rl := middleware.NewRateLimiter(20, 40)
	r.Use(rl.Middleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(deps.Config, deps.UserRepo, deps.EncryptKey)
	appHandler := handlers.NewAppHandler(deps.AppRepo, deps.UserRepo, deps.Container, deps.BuildQueue, deps.EncryptKey)
	deployHandler := handlers.NewDeploymentHandler(deps.DeployRepo, deps.AppRepo, deps.BuildQueue)
	serviceHandler := handlers.NewServiceHandler(deps.ServiceRepo, deps.AppRepo, deps.Provisioner, deps.EncryptKey)
	metricHandler := handlers.NewMetricHandler(deps.MetricRepo, deps.AppRepo)
	alertHandler := handlers.NewAlertHandler(deps.AlertRepo, deps.AppRepo)
	adminHandler := handlers.NewAdminHandler(deps.UserRepo, deps.AppRepo, deps.DeployRepo, deps.Container)
	traefikHandler := handlers.NewTraefikHandler(deps.Router)
	webhookHandler := handlers.NewWebhookHandler(deps.WebhookSvc, deps.Config.InternalToken)

	authMiddleware := middleware.Auth(deps.Config.JWTSecret, deps.UserRepo)

	r.Route("/api", func(r chi.Router) {
		// Auth (public)
		r.Get("/auth/github", authHandler.GitHubLogin)
		r.Get("/auth/github/callback", authHandler.GitHubCallback)

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
			r.Get("/apps/{id}/logs", appHandler.ContainerLogs)

			// Deployments
			r.Get("/apps/{id}/deployments", deployHandler.List)
			r.Get("/deployments/{id}/logs", deployHandler.GetLogs)
			r.Post("/deployments/{id}/rollback", deployHandler.Rollback)

			// Services
			r.Post("/apps/{id}/services", serviceHandler.Create)
			r.Get("/apps/{id}/services", serviceHandler.List)
			r.Delete("/services/{id}", serviceHandler.Delete)

			// Metrics
			r.Get("/apps/metrics/latest", metricHandler.LatestAll)
			r.Get("/apps/{id}/metrics", metricHandler.Get)

			// Alerts
			r.Post("/apps/{id}/alerts", alertHandler.Create)
			r.Get("/apps/{id}/alerts", alertHandler.List)
			r.Patch("/alerts/{id}", alertHandler.Update)
			r.Delete("/alerts/{id}", alertHandler.Delete)

			// Admin
			r.Group(func(r chi.Router) {
				r.Use(middleware.AdminOnly)
				r.Get("/admin/users", adminHandler.ListUsers)
				r.Get("/admin/stats", adminHandler.Stats)
				r.Delete("/admin/apps/{id}", adminHandler.ForceDeleteApp)
			})
		})
	})

	return r
}
