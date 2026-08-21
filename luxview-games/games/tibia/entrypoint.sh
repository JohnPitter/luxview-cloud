#!/bin/bash
# =============================================================================
# LuxView Tibia entrypoint — supervisiona MariaDB + login-server + Canary
# dentro de um único container.
#
# Config via env (injetadas pelo LuxView engine a partir dos ConfigFields do
# template "tibia", prefixo TIBIA_*):
#   TIBIA_SERVER_NAME   nome do servidor (default "OpenTibiaBR Canary")
#   TIBIA_SERVER_IP     IP anunciado; default LUXVIEW_PUBLIC_IP
#   TIBIA_LOGIN_PORT    porta de login TCP do Canary (default 7171)
#   TIBIA_GAME_PORT     porta de jogo (default 7172)
#   TIBIA_STATUS_PORT   porta de status (default 7173)
#   TIBIA_MAX_PLAYERS   limite de jogadores (0 = sem limite)
#   TIBIA_RATE_EXP/SKILL/MAGIC/LOOT/SPAWN   rates
#   TIBIA_DEATH_LOSE_PERCENT  perda em morte (0 = nenhuma)
#   TIBIA_FREE_PREMIUM  premium gratuito (true/false)
#   TIBIA_DB_PASSWORD   senha do banco interno (default "canary")
# =============================================================================
set -Eeuo pipefail

CANARY_DIR="/canary"
DATA_PACK="data-otservbr-global"
MYSQL_DATADIR="/var/lib/mysql"
INIT_SOCKET="/tmp/tibia-mysql-init.sock"
INIT_PID="/tmp/tibia-mysql-init.pid"
MAP_PATH="$CANARY_DIR/$DATA_PACK/world/otservbr.otbm"

mysql_password="${TIBIA_DB_PASSWORD:-canary}"
root_password="${MYSQL_ROOT_PASSWORD:-root}"
public_ip="${LUXVIEW_PUBLIC_IP:-${TIBIA_SERVER_IP:-127.0.0.1}}"
server_name="${TIBIA_SERVER_NAME:-OpenTibiaBR Canary}"
login_port="${TIBIA_LOGIN_PORT:-7171}"
game_port="${TIBIA_GAME_PORT:-7172}"
status_port="${TIBIA_STATUS_PORT:-7173}"
login_http_port="${LOGIN_HTTP_PORT:-8088}"
login_grpc_port="${LOGIN_GRPC_PORT:-9090}"
map_url="${CANARY_MAP_URL:-https://github.com/opentibiabr/canary/releases/download/v3.6.1/otservbr.otbm}"
max_players="${TIBIA_MAX_PLAYERS:-0}"

rate_exp="${TIBIA_RATE_EXP:-20}"
rate_skill="${TIBIA_RATE_SKILL:-20}"
rate_magic="${TIBIA_RATE_MAGIC:-20}"
rate_loot="${TIBIA_RATE_LOOT:-5}"
rate_spawn="${TIBIA_RATE_SPAWN:-2}"
death_lose="${TIBIA_DEATH_LOSE_PERCENT:-0}"
free_premium="${TIBIA_FREE_PREMIUM:-true}"

db_pid=""
login_pid=""
canary_pid=""

log() { printf '[tibia] %s\n' "$*"; }

set_lua_line() {
    local key="$1" replacement="$2" tmp found line
    tmp=$(mktemp)
    found=0
    while IFS= read -r line || [ -n "$line" ]; do
        if [[ "$line" == "$key = "* ]]; then
            printf '%s\n' "$replacement"
            found=1
        else
            printf '%s\n' "$line"
        fi
    done <"$CANARY_DIR/config.lua" >"$tmp"
    if [ "$found" -eq 0 ]; then
        printf '%s\n' "$replacement" >>"$tmp"
    fi
    mv "$tmp" "$CANARY_DIR/config.lua"
}

set_lua_string() {
    local key="$1" value="$2"
    value=$(printf '%s' "$value" | sed 's/\\/\\\\/g; s/"/\\"/g')
    set_lua_line "$key" "$key = \"$value\""
}

set_lua_number() { set_lua_line "$1" "$1 = $2"; }

sql_escape() { printf '%s' "$1" | sed "s/'/''/g"; }

wait_for_socket() {
    local password="${1:-}"
    local i
    for i in $(seq 1 60); do
        if [[ -n "$password" ]]; then
            if mariadb --protocol=socket --socket="$INIT_SOCKET" -uroot -p"$password" -e 'SELECT 1' >/dev/null 2>&1; then
                return 0
            fi
        elif mariadb --protocol=socket --socket="$INIT_SOCKET" -uroot -e 'SELECT 1' >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    echo "MariaDB não iniciou durante a inicialização." >&2
    exit 1
}

wait_for_db() {
    local i
    for i in $(seq 1 60); do
        if (exec 3<>/dev/tcp/127.0.0.1/3306) 2>/dev/null; then
            exec 3>&- 2>/dev/null || true
            return 0
        fi
        if ! kill -0 "$db_pid" 2>/dev/null; then
            exit 1
        fi
        sleep 1
    done
    echo "Tempo esgotado aguardando MariaDB na porta 3306." >&2
    exit 1
}

start_init_server() {
    mkdir -p /run/mysqld
    chown mysql:mysql /run/mysqld
    rm -f "$INIT_SOCKET" "$INIT_PID"
    mariadbd --user=mysql --datadir="$MYSQL_DATADIR" --skip-networking \
        --socket="$INIT_SOCKET" --pid-file="$INIT_PID" >/tmp/tibia-mysql-init.log 2>&1 &
    db_pid=$!
}

stop_init_server() {
    if [[ -n "$db_pid" ]]; then
        mariadb-admin --protocol=socket --socket="$INIT_SOCKET" -uroot -p"$root_password" shutdown >/dev/null 2>&1 || kill "$db_pid" >/dev/null 2>&1 || true
        wait "$db_pid" 2>/dev/null || true
        db_pid=""
    fi
}

initialize_database() {
    log "Inicializando MariaDB (primeiro boot)..."
    # /var/lib/mysql é um volume Docker (mount point); remover o diretório falha
    # com "Device or resource busy". Só criamos/chown quando não existe e apenas
    # rodamos mariadb-install-db quando o datadir está vazio.
    if [ -d "$MYSQL_DATADIR" ] && [ -n "$(ls -A "$MYSQL_DATADIR" 2>/dev/null)" ]; then
        log "Datadir já contém dados — pulando mariadb-install-db"
    else
        mkdir -p "$MYSQL_DATADIR"
        chown -R mysql:mysql "$MYSQL_DATADIR"
        mariadb-install-db --user=mysql --datadir="$MYSQL_DATADIR" >/tmp/tibia-mysql-install.log 2>&1
    fi

    start_init_server
    wait_for_socket

    local esc_root esc_user esc_pass
    esc_root=$(sql_escape "$root_password")
    esc_user=$(sql_escape "canary")
    esc_pass=$(sql_escape "$mysql_password")

    mariadb --protocol=socket --socket="$INIT_SOCKET" -uroot <<SQL
ALTER USER 'root'@'localhost' IDENTIFIED BY '$esc_root';
CREATE USER 'root'@'%' IDENTIFIED BY '$esc_root';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION;
CREATE DATABASE IF NOT EXISTS canary CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'canary'@'%' IDENTIFIED BY '$esc_pass';
CREATE USER 'canary'@'localhost' IDENTIFIED BY '$esc_pass';
GRANT ALL PRIVILEGES ON canary.* TO 'canary'@'%';
GRANT ALL PRIVILEGES ON canary.* TO 'canary'@'localhost';
FLUSH PRIVILEGES;
SQL

    stop_init_server
    log "Importando schema do Canary..."
    start_init_server
    wait_for_socket "$root_password"
    mariadb --protocol=socket --socket="$INIT_SOCKET" -uroot -p"$root_password" canary <"$CANARY_DIR/schema.sql"
    stop_init_server
    touch "$MYSQL_DATADIR/.canary-imported"
}

start_database() {
    mkdir -p /run/mysqld
    chown mysql:mysql /run/mysqld
    mariadbd --user=mysql --datadir="$MYSQL_DATADIR" --bind-address=0.0.0.0 \
        --port=3306 --socket=/run/mysqld/mysqld.sock >>/tmp/tibia-mariadb.log 2>&1 &
    db_pid=$!
}

ensure_database_user() {
    local esc_user esc_pass
    esc_user=$(sql_escape "canary")
    esc_pass=$(sql_escape "$mysql_password")
    mariadb -h 127.0.0.1 -P 3306 -uroot -p"$root_password" <<SQL
CREATE USER IF NOT EXISTS 'canary'@'%' IDENTIFIED BY '$esc_pass';
ALTER USER 'canary'@'%' IDENTIFIED BY '$esc_pass';
CREATE USER IF NOT EXISTS 'canary'@'localhost' IDENTIFIED BY '$esc_pass';
ALTER USER 'canary'@'localhost' IDENTIFIED BY '$esc_pass';
GRANT ALL PRIVILEGES ON canary.* TO 'canary'@'%';
GRANT ALL PRIVILEGES ON canary.* TO 'canary'@'localhost';
FLUSH PRIVILEGES;
SQL
}

download_map() {
    if [ -f "$MAP_PATH" ]; then
        log "Mapa já existe, download pulado"
        return 0
    fi
    log "Baixando mapa..."
    mkdir -p "$(dirname "$MAP_PATH")"
    tmp_map="$MAP_PATH.tmp"
    rm -f "$tmp_map"
    if ! curl --fail --show-error --location --connect-timeout 5 --max-time 600 "$map_url" -o "$tmp_map"; then
        rm -f "$tmp_map"
        echo "Falha ao baixar o mapa." >&2
        exit 1
    fi
    mv "$tmp_map" "$MAP_PATH"
    log "Mapa baixado"
}

configure_canary() {
    log "Aplicando configuração do Canary..."
    cd "$CANARY_DIR"
    set_lua_string "ip" "$public_ip"
    set_lua_string "serverName" "$server_name"
    set_lua_number "loginProtocolPort" "$login_port"
    set_lua_number "gameProtocolPort" "$game_port"
    set_lua_number "statusProtocolPort" "$status_port"
    set_lua_number "maxPlayers" "$max_players"
    set_lua_string "mysqlHost" "127.0.0.1"
    set_lua_string "mysqlUser" "canary"
    set_lua_string "mysqlPass" "$mysql_password"
    set_lua_string "mysqlDatabase" "canary"
    set_lua_string "dataPackDirectory" "$DATA_PACK"
    set_lua_number "rateExp" "$rate_exp"
    set_lua_number "rateSkill" "$rate_skill"
    set_lua_number "rateMagic" "$rate_magic"
    set_lua_number "rateLoot" "$rate_loot"
    set_lua_number "rateSpawn" "$rate_spawn"
    set_lua_number "deathLosePercent" "$death_lose"
    set_lua_line "freePremium" "freePremium = $free_premium"
    log "config.lua atualizado"
}

start_login_server() {
    log "Iniciando login-server (HTTP :$login_http_port)..."
    LOGIN_HTTP_PORT="$login_http_port" \
    LOGIN_GRPC_PORT="$login_grpc_port" \
    MYSQL_HOST="127.0.0.1" \
    MYSQL_PORT="3306" \
    MYSQL_DBNAME="canary" \
    MYSQL_USER="canary" \
    MYSQL_PASS="$mysql_password" \
    SERVER_NAME="$server_name" \
    SERVER_IP="$public_ip" \
    SERVER_PORT="$game_port" \
    SERVER_LOCATION="BRA" \
    RATE_LIMITER_RATE="${LOGIN_RATE_LIMITER_RATE:-2}" \
    RATE_LIMITER_BURST="${LOGIN_RATE_LIMITER_BURST:-5}" \
    /usr/local/bin/login-server >>/tmp/tibia-login-server.log 2>&1 &
    login_pid=$!
}

start_canary() {
    log "Iniciando Canary..."
    cd "$CANARY_DIR"
    canary >>/tmp/tibia-canary.log 2>&1 &
    canary_pid=$!
}

cleanup() {
    trap - EXIT INT TERM
    local pid
    for pid in "$canary_pid" "$login_pid" "$db_pid"; do
        [[ -n "$pid" ]] && kill -TERM "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
}

trap cleanup EXIT INT TERM

if [[ ! -f "$MYSQL_DATADIR/.canary-imported" ]]; then
    initialize_database
fi

start_database
wait_for_db
ensure_database_user
download_map
configure_canary
start_login_server
start_canary

log "Tibia (Canary) no ar — jogo :$game_port | login :$login_http_port"
while true; do
    local_state=""
    for name in db_pid login_pid canary_pid; do
        pid="${!name}"
        state=""
        if [[ -n "$pid" ]]; then
            state="$(ps -o stat= -p "$pid" 2>/dev/null || true)"
        fi
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null && [[ "$state" != Z* ]]; then
            continue
        fi
        wait "$pid" 2>/dev/null || true
        log "${name} encerrou; reiniciando"
        case "$name" in
            db_pid) start_database; wait_for_db; ensure_database_user ;;
            login_pid) start_login_server ;;
            canary_pid) start_canary ;;
        esac
    done
    sleep 3
done
