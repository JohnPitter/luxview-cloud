# Self-Hosting Guide

Deploy LuxView Cloud on your own VPS.

## Requirements

| Requirement | Minimum | Recommended |
|---|---|---|
| OS | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 40 GB SSD | 80 GB SSD |
| Domain | 1 domain with wildcard DNS | — |

You also need a **GitHub OAuth App** for authentication.

## Step 1: Prepare the VPS

SSH into your VPS as root and run the setup script:

```bash
git clone https://github.com/JohnPitter/luxview-cloud.git /opt/luxview-cloud
cd /opt/luxview-cloud
bash scripts/setup-vps.sh
```

This script:

- Updates system packages
- Installs Docker Engine and Docker Compose
- Configures UFW firewall (SSH, HTTP, HTTPS only)
- Creates a 2 GB swap file
- Tunes kernel parameters for Docker networking
- Creates a `luxview` system user
- Sets up fail2ban for SSH brute-force protection
- Configures Docker log rotation

## Step 2: Configure DNS

Point your domain to the VPS IP address. You need **two DNS records**:

| Type | Name | Value |
|---|---|---|
| A | `luxview.cloud` | `<your-vps-ip>` |
| A | `*.luxview.cloud` | `<your-vps-ip>` |

Replace `luxview.cloud` with your actual domain.

> **Important:** The wildcard record (`*`) is essential. Every deployed app gets a subdomain that must resolve to your VPS.

Allow 5–10 minutes for DNS propagation before proceeding.

## Step 3: Create a GitHub OAuth App

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Click **New OAuth App**
3. Fill in:
   - **Application name:** LuxView Cloud
   - **Homepage URL:** `https://luxview.cloud`
   - **Authorization callback URL:** `https://luxview.cloud/api/auth/github/callback`
4. Click **Register application**
5. Copy the **Client ID**
6. Generate a **Client Secret** and copy it

## Step 4: Configure Environment

```bash
cd /opt/luxview-cloud
cp .env.example .env
vim .env
```

Fill in all `CHANGE_ME` values:

```bash
# Your domain
DOMAIN=luxview.cloud

# Platform database password (generate a strong one)
DB_PASSWORD=<random-32-char-string>

# Encryption key for credentials at rest (min 32 characters)
ENCRYPTION_KEY=<random-32-char-string>

# JWT secret (random string)
JWT_SECRET=<random-32-char-string>

# GitHub OAuth (from Step 3)
GITHUB_CLIENT_ID=<your-client-id>
GITHUB_CLIENT_SECRET=<your-client-secret>

# Shared service passwords (generate unique ones)
SHARED_PG_PASSWORD=<random-24-char-string>
SHARED_REDIS_PASSWORD=<random-24-char-string>
SHARED_MONGO_PASSWORD=<random-24-char-string>
SHARED_RABBITMQ_PASSWORD=<random-24-char-string>
SHARED_MINIO_PASSWORD=<random-24-char-string>

# Let's Encrypt (your email for SSL cert notifications)
ACME_EMAIL=admin@luxview.cloud
```

> **Tip:** Generate random passwords with: `openssl rand -base64 32`

## Step 5: Start the Platform

```bash
make prod
```

This starts all services in detached mode:

- **Traefik** — reverse proxy with automatic SSL
- **LuxView Engine** — Go API backend
- **Dashboard** — React SPA served by Nginx
- **PostgreSQL (platform)** — stores users, apps, deployments
- **PostgreSQL (shared)** — user app databases
- **Redis, MongoDB, RabbitMQ, MinIO** — shared services for user apps

## Step 6: Run Migrations

```bash
make migrate
```

This creates all necessary database tables in the platform PostgreSQL.

## Step 7: Verify

Check that all services are running:

```bash
make status
```

You should see all containers with status `Up`:

```
luxview-traefik       Up    0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
luxview-engine        Up    8080/tcp
luxview-dashboard     Up    80/tcp
luxview-pg-platform   Up (healthy)
luxview-pg-shared     Up (healthy)
luxview-redis-shared  Up (healthy)
luxview-mongo-shared  Up (healthy)
luxview-minio-shared  Up (healthy)
luxview-rabbitmq-shared  Up (healthy)
```

Visit `https://luxview.cloud` — you should see the landing page.

## Deploying Updates

Use the deploy script for zero-downtime updates:

```bash
bash scripts/deploy.sh
```

This script:

1. Pulls the latest code from `main`
2. Builds new Docker images
3. Runs database migrations
4. Restarts the engine (waits for health check)
5. Restarts the dashboard
6. Verifies all services are running

## Backups

Run a database backup:

```bash
make backup
```

This executes `scripts/backup.sh`, which dumps all databases to `/backups/`.

## Useful Commands

| Command | Description |
|---|---|
| `make status` | Show running containers |
| `make logs` | Follow all logs |
| `make log-service SVC=engine` | Follow specific service logs |
| `make psql` | Connect to platform database |
| `make psql-shared` | Connect to shared database |
| `make restart` | Restart all services |
| `make backup` | Backup databases |
| `make stats` | Show CPU/memory per container |
| `make shell SVC=engine` | Shell into a container |

## Traefik Configuration

Traefik handles:

- **Automatic SSL** via Let's Encrypt (ACME HTTP challenge)
- **Wildcard routing** — `*.luxview.cloud` routes to user app containers
- **Rate limiting** — built into the engine middleware

The Traefik configuration is at `traefik/traefik.yml` and dynamic config in `traefik/dynamic/`.

## Troubleshooting

### SSL certificates not provisioning

- Verify DNS records are propagated: `dig +short luxview.cloud` and `dig +short test.luxview.cloud`
- Check Traefik logs: `make log-service SVC=traefik`
- Ensure ports 80 and 443 are open: `ufw status`

### Engine health check failing

- Check engine logs: `make log-service SVC=engine`
- Verify database is accessible: `make psql` then `\l`
- Check `.env` values are correct

### Apps not accessible

- Verify the wildcard DNS record exists
- Check the app container is running: `docker ps | grep <app-name>`
- Check Traefik routing: `curl -s http://localhost/api/internal/traefik-config` (internal endpoint)

### Out of disk space

- Check disk usage: `df -h`
- Clean unused Docker images: `docker system prune -a`
- Check build cache: `docker volume ls`
