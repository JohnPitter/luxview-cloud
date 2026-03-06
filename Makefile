# =============================================================================
# LuxView Cloud Platform — Makefile
# =============================================================================

.PHONY: dev prod build logs migrate clean status stop restart backup

COMPOSE = docker compose
COMPOSE_DEV = $(COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml
MIGRATIONS_DIR = luxview-engine/migrations

# -- Development --------------------------------------------------------------

## Start all services in development mode (hot reload, exposed ports)
dev:
	$(COMPOSE_DEV) up --build

## Start dev in background
dev-detach:
	$(COMPOSE_DEV) up --build -d

# -- Production ---------------------------------------------------------------

## Start all services in production mode (detached)
prod:
	$(COMPOSE) up -d

## Rebuild and start production
prod-build:
	$(COMPOSE) up -d --build

# -- Build --------------------------------------------------------------------

## Build all images without starting
build:
	$(COMPOSE) build

## Build without cache
build-fresh:
	$(COMPOSE) build --no-cache

# -- Logs ---------------------------------------------------------------------

## Follow logs for all services
logs:
	$(COMPOSE) logs -f

## Follow logs for a specific service (usage: make log-service SVC=engine)
log-service:
	$(COMPOSE) logs -f $(SVC)

# -- Migrations ---------------------------------------------------------------

## Run all SQL migrations against pg-platform
migrate:
	@echo "Running migrations against pg-platform..."
	@for f in $(MIGRATIONS_DIR)/*.sql; do \
		echo "  -> $$(basename $$f)"; \
		$(COMPOSE) exec -T pg-platform psql -U luxview -d luxview_platform -f /dev/stdin < "$$f"; \
	done
	@echo "Migrations complete."

## Run migrations in dev mode
migrate-dev:
	@echo "Running migrations against pg-platform (dev)..."
	@for f in $(MIGRATIONS_DIR)/*.sql; do \
		echo "  -> $$(basename $$f)"; \
		$(COMPOSE_DEV) exec -T pg-platform psql -U luxview -d luxview_platform -f /dev/stdin < "$$f"; \
	done
	@echo "Migrations complete."

# -- Lifecycle ----------------------------------------------------------------

## Stop all services
stop:
	$(COMPOSE) down

## Restart all services
restart:
	$(COMPOSE) restart

## Stop and remove all containers, networks, and volumes (DESTRUCTIVE)
clean:
	$(COMPOSE) down -v --remove-orphans
	@echo "All containers, networks, and volumes removed."

# -- Status -------------------------------------------------------------------

## Show running containers and their status
status:
	$(COMPOSE) ps

## Show resource usage per container
stats:
	docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"

# -- Backup -------------------------------------------------------------------

## Backup databases to ./backups/
backup:
	@bash scripts/backup.sh

# -- Helpers ------------------------------------------------------------------

## Shell into a service container (usage: make shell SVC=engine)
shell:
	$(COMPOSE) exec $(SVC) sh

## Connect to platform database
psql:
	$(COMPOSE) exec pg-platform psql -U luxview -d luxview_platform

## Connect to shared database
psql-shared:
	$(COMPOSE) exec pg-shared psql -U luxview_admin -d luxview_shared

## Print help
help:
	@echo "LuxView Cloud Platform"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' Makefile | sed 's/^## /  /'
