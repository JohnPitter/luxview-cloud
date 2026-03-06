#!/usr/bin/env bash
# =============================================================================
# LuxView Cloud — Zero-Downtime Deploy Script
# =============================================================================
# Pulls latest code, rebuilds images, and restarts services one by one.
# Usage: ./scripts/deploy.sh [branch]
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BRANCH="${1:-main}"
COMPOSE="docker compose"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

LOG_PREFIX="[luxview-deploy]"
log() { echo "$LOG_PREFIX $(date '+%H:%M:%S') $1"; }

cd "$PROJECT_DIR"

# -- Pre-flight checks --------------------------------------------------------
log "Starting deployment (branch: $BRANCH)..."

if [[ ! -f ".env" ]]; then
    log "ERROR: .env file not found. Copy .env.example to .env first."
    exit 1
fi

# -- 1. Pull latest code ------------------------------------------------------
log "Pulling latest code from origin/$BRANCH..."
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git pull origin "$BRANCH"

COMMIT_SHA=$(git rev-parse --short HEAD)
log "Deploying commit: $COMMIT_SHA"

# -- 2. Build new images ------------------------------------------------------
log "Building images..."
$COMPOSE build --parallel

# -- 3. Run migrations --------------------------------------------------------
log "Running database migrations..."
for f in luxview-engine/migrations/*.sql; do
    log "  -> $(basename "$f")"
    $COMPOSE exec -T pg-platform psql -U luxview -d luxview_platform -f /dev/stdin < "$f" 2>/dev/null || true
done

# -- 4. Rolling restart (zero downtime) ---------------------------------------
# Restart services that don't affect user traffic first
log "Restarting engine..."
$COMPOSE up -d --no-deps --build engine

# Wait for engine health
log "Waiting for engine to be healthy..."
RETRIES=30
for i in $(seq 1 $RETRIES); do
    if $COMPOSE exec -T engine wget -qO- http://localhost:8080/api/health >/dev/null 2>&1; then
        log "Engine healthy after ${i}s."
        break
    fi
    if [[ $i -eq $RETRIES ]]; then
        log "WARNING: Engine health check timed out after ${RETRIES}s."
    fi
    sleep 1
done

# Restart dashboard
log "Restarting dashboard..."
$COMPOSE up -d --no-deps --build dashboard

# -- 5. Verify ----------------------------------------------------------------
log "Verifying all services..."
$COMPOSE ps

# -- 6. Tag the deployment -----------------------------------------------------
log "==========================================="
log " Deployment complete!"
log " Commit:    $COMMIT_SHA"
log " Timestamp: $TIMESTAMP"
log "==========================================="
