#!/usr/bin/env bash
# =============================================================================
# LuxView Cloud — Database Backup Script
# =============================================================================
# Backs up pg-platform, pg-shared, and MongoDB to /backups/.
# Retains backups for 30 days. Run via cron daily.
#
# Crontab example (daily at 3 AM):
#   0 3 * * * /opt/luxview-cloud/scripts/backup.sh >> /var/log/luxview-backup.log 2>&1
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="/backups"
RETENTION_DAYS=30
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
COMPOSE="docker compose"

LOG_PREFIX="[luxview-backup]"
log() { echo "$LOG_PREFIX $(date '+%Y-%m-%d %H:%M:%S') $1"; }

cd "$PROJECT_DIR"

# Load env vars for passwords
if [[ -f ".env" ]]; then
    set -a
    source .env
    set +a
fi

mkdir -p "$BACKUP_DIR"

# -- 1. PostgreSQL Platform ---------------------------------------------------
log "Backing up pg-platform..."
PG_PLATFORM_FILE="$BACKUP_DIR/pg-platform_${TIMESTAMP}.sql.gz"
$COMPOSE exec -T pg-platform pg_dumpall -U luxview | gzip > "$PG_PLATFORM_FILE"
log "  -> $PG_PLATFORM_FILE ($(du -h "$PG_PLATFORM_FILE" | cut -f1))"

# -- 2. PostgreSQL Shared (all user databases) --------------------------------
log "Backing up pg-shared..."
PG_SHARED_FILE="$BACKUP_DIR/pg-shared_${TIMESTAMP}.sql.gz"
$COMPOSE exec -T pg-shared pg_dumpall -U luxview_admin | gzip > "$PG_SHARED_FILE"
log "  -> $PG_SHARED_FILE ($(du -h "$PG_SHARED_FILE" | cut -f1))"

# -- 3. MongoDB Shared --------------------------------------------------------
log "Backing up mongo-shared..."
MONGO_DIR="$BACKUP_DIR/mongo-shared_${TIMESTAMP}"
$COMPOSE exec -T mongo-shared mongodump \
    -u luxview_admin \
    -p "${SHARED_MONGO_PASSWORD:-}" \
    --authenticationDatabase admin \
    --archive | gzip > "${MONGO_DIR}.archive.gz"
log "  -> ${MONGO_DIR}.archive.gz ($(du -h "${MONGO_DIR}.archive.gz" | cut -f1))"

# -- 4. Redis Shared (RDB snapshot) ------------------------------------------
log "Backing up redis-shared..."
REDIS_FILE="$BACKUP_DIR/redis-shared_${TIMESTAMP}.rdb"
$COMPOSE exec -T redis-shared redis-cli -a "${SHARED_REDIS_PASSWORD:-}" BGSAVE >/dev/null 2>&1 || true
sleep 2
docker cp luxview-redis-shared:/data/appendonlydir "$BACKUP_DIR/redis-shared_${TIMESTAMP}_aof" 2>/dev/null || \
    docker cp luxview-redis-shared:/data/dump.rdb "$REDIS_FILE" 2>/dev/null || \
    log "  WARNING: Could not copy Redis data file."
log "  -> Redis backup saved."

# -- 5. Cleanup old backups ---------------------------------------------------
log "Cleaning backups older than ${RETENTION_DAYS} days..."
DELETED=$(find "$BACKUP_DIR" -type f -mtime "+$RETENTION_DAYS" -print -delete | wc -l)
DELETED_DIRS=$(find "$BACKUP_DIR" -type d -mtime "+$RETENTION_DAYS" -empty -print -delete 2>/dev/null | wc -l)
log "  Removed $((DELETED + DELETED_DIRS)) old backup files/dirs."

# -- Summary -------------------------------------------------------------------
TOTAL_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)
log "==========================================="
log " Backup complete!"
log " Timestamp:  $TIMESTAMP"
log " Location:   $BACKUP_DIR"
log " Total size: $TOTAL_SIZE"
log "==========================================="
