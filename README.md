# LuxView Cloud

LuxView Cloud is a self-hosted PaaS (Platform as a Service) running on `luxview.cloud`. Users connect their GitHub account, select a repository, and deploy with one click. The platform auto-detects the tech stack (Node.js, Python, Go, Rust, static, or Dockerfile), builds a Docker image, starts an isolated container, and provisions a subdomain with automatic SSL -- all on a single VPS.

## Architecture

```
                        Internet
                           |
                    +------+------+
                    |   Traefik   |  :80 / :443
                    |  (SSL+Route)|  *.luxview.cloud
                    +------+------+
                           |
              +------------+------------+
              |                         |
     luxview.cloud/api/*       luxview.cloud
     +--------+--------+      +--------+--------+
     |  LuxView Engine |      |    Dashboard    |
     |      (Go)       |      |   (React SPA)   |
     |    :8080         |      |   Nginx / Vite  |
     +--------+--------+      +-----------------+
              |
     +--------+--------+
     |  Docker Engine   |   User app containers
     |  [A1] [A2] [A3]  |   <app>.luxview.cloud
     +--------+--------+
              |
     +--------+--------+--------+--------+
     |  pg-platform  |  pg-shared  | redis |
     |  (platform DB)|  (user DBs) | mongo |
     |               |             | rabbit|
     +--------------+--------------+------+
```

## Quick Start (Development)

```bash
# 1. Clone the repository
git clone <repo-url> luxview-cloud && cd luxview-cloud

# 2. Set up environment
cp .env.example .env
# Edit .env with your GitHub OAuth credentials (GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET)

# 3. Start all services
make dev

# 4. Run database migrations (in another terminal, after services are up)
make migrate-dev

# 5. Access
#    Dashboard:       http://localhost
#    Traefik panel:   http://localhost:8080
#    Engine API:      http://localhost/api/health
#    PostgreSQL:      localhost:5432 (platform) / :5433 (shared)
#    Redis:           localhost:6379
#    MongoDB:         localhost:27017
#    RabbitMQ UI:     localhost:15672
```

## Production Deployment

```bash
# 1. Set up the VPS (Ubuntu 22.04, run as root)
bash scripts/setup-vps.sh

# 2. Clone to /opt/luxview-cloud
git clone <repo-url> /opt/luxview-cloud && cd /opt/luxview-cloud

# 3. Configure environment
cp .env.example .env
vim .env  # Fill ALL values with strong secrets

# 4. DNS: Point luxview.cloud + *.luxview.cloud to VPS IP

# 5. Start
make prod

# 6. Run migrations
make migrate

# 7. Deploy updates (zero downtime)
bash scripts/deploy.sh main
```

## Environment Variables

| Variable | Description | Required |
|---|---|---|
| `DOMAIN` | Platform domain (default: `luxview.cloud`) | Yes |
| `DB_PASSWORD` | Platform PostgreSQL password | Yes |
| `ENCRYPTION_KEY` | AES-256-GCM key (min 32 chars) | Yes |
| `JWT_SECRET` | JWT signing secret | Yes |
| `GITHUB_CLIENT_ID` | GitHub OAuth App client ID | Yes |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret | Yes |
| `SHARED_PG_PASSWORD` | Shared PostgreSQL password | Yes |
| `SHARED_REDIS_PASSWORD` | Shared Redis password | Yes |
| `SHARED_MONGO_PASSWORD` | Shared MongoDB password | Yes |
| `SHARED_RABBITMQ_USER` | RabbitMQ admin user (default: `luxview_admin`) | No |
| `SHARED_RABBITMQ_PASSWORD` | Shared RabbitMQ password | Yes |
| `ACME_EMAIL` | Let's Encrypt notification email | Prod |
| `ENGINE_PORT` | Engine listen port (default: `8080`) | No |
| `BUILD_CONCURRENCY` | Max concurrent builds (default: `3`) | No |
| `LOG_LEVEL` | Log level: debug, info, warn, error | No |

## Directory Structure

```
luxview-cloud/
  docker-compose.yml          # Production compose
  docker-compose.dev.yml      # Development override
  .env.example                # Environment template
  Makefile                    # Common commands
  traefik/
    traefik.yml               # Production Traefik config
    traefik.dev.yml           # Development Traefik config
    dynamic/                  # Dynamic middleware config
  luxview-engine/             # Go API (the brain)
    cmd/engine/main.go
    internal/                 # Handlers, services, repos, workers
    pkg/                      # Shared packages (crypto, docker, logger)
    migrations/               # SQL migration files
  luxview-dashboard/          # React SPA
    src/
    nginx.conf                # Production Nginx config
  scripts/
    setup-vps.sh              # Initial VPS provisioning
    deploy.sh                 # Zero-downtime deploy
    backup.sh                 # Database backup (cron)
  docs/
    plans/                    # Design documents
```

## Make Commands

| Command | Description |
|---|---|
| `make dev` | Start dev environment (hot reload, exposed ports) |
| `make prod` | Start production (detached) |
| `make build` | Build all Docker images |
| `make logs` | Follow all service logs |
| `make migrate` | Run SQL migrations |
| `make status` | Show running containers |
| `make backup` | Backup all databases |
| `make clean` | Stop and remove everything (volumes included) |
| `make psql` | Connect to platform database |
| `make shell SVC=engine` | Shell into a container |
