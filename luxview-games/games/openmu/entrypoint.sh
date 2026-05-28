#!/bin/bash
set -e

DATA_DIR="/openmu-data"
PGDATA="$DATA_DIR/pgdata"

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-openmu}"
OPENMU_ADMIN_USER="${OPENMU_ADMIN_USER:-admin}"
OPENMU_ADMIN_PASS="${OPENMU_ADMIN_PASS:-openmu}"

mkdir -p "$DATA_DIR"

# Initialize PostgreSQL data directory if first run
if [ ! -f "$PGDATA/PG_VERSION" ]; then
    echo "[openmu] Initializing PostgreSQL database..."
    mkdir -p "$PGDATA"
    chown -R postgres:postgres "$PGDATA"
    su - postgres -c "/usr/lib/postgresql/16/bin/initdb -D $PGDATA --encoding=UTF8 --locale=C"

    # Allow local connections with password
    echo "host all all 127.0.0.1/32 md5" >> "$PGDATA/pg_hba.conf"
    echo "host all all ::1/128 md5" >> "$PGDATA/pg_hba.conf"
    sed -i "s/#listen_addresses = 'localhost'/listen_addresses = 'localhost'/" "$PGDATA/postgresql.conf"

    # Start PostgreSQL temporarily to create the database and user
    su - postgres -c "/usr/lib/postgresql/16/bin/pg_ctl -D $PGDATA -w start"
    su - postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD '$POSTGRES_PASSWORD';\""
    su - postgres -c "psql -c \"CREATE DATABASE openmu;\""
    su - postgres -c "/usr/lib/postgresql/16/bin/pg_ctl -D $PGDATA -w stop"

    echo "[openmu] PostgreSQL initialized."
else
    chown -R postgres:postgres "$PGDATA"
fi

echo "[openmu] Starting services via supervisord..."
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/openmu.conf
