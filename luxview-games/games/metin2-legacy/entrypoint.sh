#!/usr/bin/env bash
set -Eeuo pipefail

data_dir="${MYSQL_DATADIR:-/var/lib/mysql}"
legacy_dir="/opt/openmetin/legacy"
log_dir="/var/log/openmetin"
init_socket="/tmp/openmetin-mysql-init.sock"
init_pid="/tmp/openmetin-mysql-init.pid"
db_pid=""

mysql_host="${METIN_MYSQL_HOST:-127.0.0.1}"
mysql_port="${METIN_MYSQL_PORT:-3306}"
mysql_user="${METIN_DB_USER:-user}"
mysql_password="${METIN_DB_PASSWORD:-pw}"
root_password="${MYSQL_ROOT_PASSWORD:-root}"
public_ip="${LUXVIEW_PUBLIC_IP:-${METIN_PUBLIC_IP:-127.0.0.1}}"
bind_ip="${METIN_BIND_IP:-127.0.0.1}"
process_names=()
declare -A process_directories process_executables process_pids

log() {
    printf '[openmetin] %s\n' "$*"
}

validate_config_value() {
    local name="$1"
    local value="$2"
    if [[ ! "$value" =~ ^[A-Za-z0-9._@%-]+$ ]]; then
        echo "Valor inválido em ${name}. Use apenas letras, números e . _ @ % -." >&2
        exit 1
    fi
}

sql_escape() {
    printf '%s' "$1" | sed "s/'/''/g"
}

wait_for_socket() {
    local password="${1:-}"
    for _ in $(seq 1 60); do
        if [[ -n "$password" ]]; then
            if mariadb --protocol=socket --socket="$init_socket" -uroot -p"$password" -e 'SELECT 1' >/dev/null 2>&1; then
                return 0
            fi
        elif mariadb --protocol=socket --socket="$init_socket" -uroot -e 'SELECT 1' >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    echo 'MariaDB não iniciou durante a inicialização.' >&2
    cat /tmp/openmetin-mysql-init.log >&2 || true
    exit 1
}

start_init_server() {
    mkdir -p /run/mysqld
    chown mysql:mysql /run/mysqld
    rm -f "$init_socket" "$init_pid"
    mariadbd --user=mysql --datadir="$data_dir" --skip-networking \
        --socket="$init_socket" --pid-file="$init_pid" "$@" \
        >/tmp/openmetin-mysql-init.log 2>&1 &
    db_pid=$!
}

stop_database() {
    if [[ -z "$db_pid" ]]; then
        return 0
    fi
    mariadb-admin --protocol=socket --socket="$init_socket" -uroot -p"$root_password" shutdown >/dev/null 2>&1 || kill "$db_pid" >/dev/null 2>&1 || true
    wait "$db_pid" 2>/dev/null || true
    db_pid=""
}

repair_legacy_tables() {
    mapfile -t repair_statements < <(
        mariadb --protocol=socket --socket="$init_socket" -uroot -p"$root_password" -N -B -e \
            "SELECT CONCAT('REPAIR TABLE ', TABLE_SCHEMA, '.', TABLE_NAME, ';') FROM information_schema.tables WHERE TABLE_SCHEMA IN ('account', 'common', 'log', 'player') AND ENGINE = 'MyISAM'"
    )
    for statement in "${repair_statements[@]}"; do
        mariadb --protocol=socket --socket="$init_socket" -uroot -p"$root_password" -e "$statement"
    done
}

initialize_legacy_databases() {
    log 'Inicializando MariaDB e importando os bancos legados...'
    mkdir -p "$data_dir"
    chown -R mysql:mysql "$data_dir"
    mariadb-install-db --user=mysql --datadir="$data_dir" >/tmp/openmetin-mysql-install.log 2>&1

    start_init_server
    wait_for_socket

    escaped_root_password="$(sql_escape "$root_password")"
    escaped_db_user="$(sql_escape "$mysql_user")"
    escaped_db_password="$(sql_escape "$mysql_password")"

    mariadb --protocol=socket --socket="$init_socket" -uroot <<SQL
ALTER USER 'root'@'localhost' IDENTIFIED BY '$escaped_root_password';
CREATE USER 'root'@'%' IDENTIFIED BY '$escaped_root_password';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION;
CREATE DATABASE IF NOT EXISTS account;
CREATE DATABASE IF NOT EXISTS common;
CREATE DATABASE IF NOT EXISTS log;
CREATE DATABASE IF NOT EXISTS player;
CREATE DATABASE IF NOT EXISTS hotbackup;
CREATE USER '$escaped_db_user'@'%' IDENTIFIED BY '$escaped_db_password';
GRANT ALL PRIVILEGES ON account.* TO '$escaped_db_user'@'%';
GRANT ALL PRIVILEGES ON common.* TO '$escaped_db_user'@'%';
GRANT ALL PRIVILEGES ON log.* TO '$escaped_db_user'@'%';
GRANT ALL PRIVILEGES ON player.* TO '$escaped_db_user'@'%';
GRANT ALL PRIVILEGES ON hotbackup.* TO '$escaped_db_user'@'%';
FLUSH PRIVILEGES;
SQL
    stop_database

    for database in account common log player hotbackup; do
        rm -rf "$data_dir/$database"
        mkdir -p "$data_dir/$database"
        cp -a "$legacy_dir/$database/." "$data_dir/$database/"
    done
    chown -R mysql:mysql "$data_dir"

    start_init_server --sql-mode=NO_ENGINE_SUBSTITUTION
    wait_for_socket "$root_password"
    repair_legacy_tables
    mariadb --protocol=socket --socket="$init_socket" -uroot -p"$root_password" <<SQL
ALTER TABLE account.account ALTER language SET DEFAULT 2;
UPDATE account.account SET language = 2;
SQL
    stop_database
    touch "$data_dir/.openmetin-legacy-imported"
}

start_database() {
    mkdir -p /run/mysqld
    chown mysql:mysql /run/mysqld
    mariadbd --user=mysql --datadir="$data_dir" --bind-address=0.0.0.0 \
        --port="$mysql_port" --sql-mode=NO_ENGINE_SUBSTITUTION \
        >>"$log_dir/mariadb.log" 2>&1 &
    db_pid=$!
}

wait_for_database() {
    for _ in $(seq 1 60); do
        if nc -z 127.0.0.1 "$mysql_port"; then
            return 0
        fi
        if ! kill -0 "$db_pid" 2>/dev/null; then
            tail -n 100 "$log_dir/mariadb.log" >&2 || true
            exit 1
        fi
        sleep 1
    done
    echo "Tempo esgotado aguardando MariaDB na porta ${mysql_port}." >&2
    exit 1
}

ensure_database_user() {
    local escaped_user escaped_password
    escaped_user="$(sql_escape "$mysql_user")"
    escaped_password="$(sql_escape "$mysql_password")"
    mariadb -h 127.0.0.1 -P "$mysql_port" -uroot -p"$root_password" <<SQL
CREATE USER IF NOT EXISTS '$escaped_user'@'%' IDENTIFIED BY '$escaped_password';
ALTER USER '$escaped_user'@'%' IDENTIFIED BY '$escaped_password';
GRANT ALL PRIVILEGES ON account.* TO '$escaped_user'@'%';
GRANT ALL PRIVILEGES ON common.* TO '$escaped_user'@'%';
GRANT ALL PRIVILEGES ON log.* TO '$escaped_user'@'%';
GRANT ALL PRIVILEGES ON player.* TO '$escaped_user'@'%';
GRANT ALL PRIVILEGES ON hotbackup.* TO '$escaped_user'@'%';
FLUSH PRIVILEGES;
SQL
}

configure_game() {
    validate_config_value METIN_MYSQL_HOST "$mysql_host"
    validate_config_value METIN_MYSQL_PORT "$mysql_port"
    validate_config_value METIN_DB_USER "$mysql_user"
    validate_config_value METIN_DB_PASSWORD "$mysql_password"
    validate_config_value LUXVIEW_PUBLIC_IP "$public_ip"
    validate_config_value METIN_BIND_IP "$bind_ip"

    sed -E -i \
        -e "s#^SQL_ACCOUNT =.*#SQL_ACCOUNT = \"${mysql_host} account ${mysql_user} ${mysql_password} ${mysql_port}\"#" \
        -e "s#^SQL_PLAYER =.*#SQL_PLAYER = \"${mysql_host} player ${mysql_user} ${mysql_password} ${mysql_port}\"#" \
        -e "s#^SQL_COMMON =.*#SQL_COMMON = \"${mysql_host} common ${mysql_user} ${mysql_password} ${mysql_port}\"#" \
        -e "s#^SQL_HOTBACKUP =.*#SQL_HOTBACKUP = \"${mysql_host} hotbackup ${mysql_user} ${mysql_password} ${mysql_port}\"#" \
        /usr/game/core/db/conf.txt

    while IFS= read -r config; do
        sed -E -i \
            "s#^(PLAYER_SQL|COMMON_SQL|LOG_SQL): [^ ]+ [^ ]+ [^ ]+ (.+)\$#\\1: ${mysql_host} ${mysql_user} ${mysql_password} \\2#" \
            "$config"
        if grep -qi '^BIND_IP:' "$config"; then
            sed -E -i "s#^BIND_IP:.*#BIND_IP: ${bind_ip}#I" "$config"
        else
            printf '\nBIND_IP: %s\n' "$bind_ip" >>"$config"
        fi
    done < <(find /usr/game/core -name CONFIG -type f)
}

start_process() {
    local name="$1"
    local directory="$2"
    local executable="$3"
    log "iniciando ${name}"
    (
        cd "$directory"
        exec "$executable"
    ) >>"$log_dir/${name}.log" 2>&1 &
    if [[ -z ${process_pids[$name]+set} ]]; then
        process_names+=("$name")
    fi
    process_directories["$name"]="$directory"
    process_executables["$name"]="$executable"
    process_pids["$name"]="$!"
}

wait_for_cache_db() {
    for _ in $(seq 1 60); do
        if nc -z 127.0.0.1 15000; then
            return 0
        fi
        sleep 1
    done
    echo 'Tempo esgotado aguardando o cache DB na porta 15000.' >&2
    tail -n 100 "$log_dir/cache-db.log" >&2 || true
    exit 1
}

stop_all() {
    trap - EXIT INT TERM
    local name pid
    for name in "${process_names[@]}"; do
        pid="${process_pids[$name]:-}"
        if [[ -n "$pid" ]]; then
            kill -TERM "$pid" 2>/dev/null || true
        fi
    done
    if [[ -n "$db_pid" ]]; then
        kill -TERM "$db_pid" 2>/dev/null || true
    fi
    wait 2>/dev/null || true
}

supervise_processes() {
    local name pid state
    for name in "${process_names[@]}"; do
        pid="${process_pids[$name]:-}"
        state=""
        if [[ -n "$pid" ]]; then
            state="$(ps -o stat= -p "$pid" 2>/dev/null || true)"
        fi
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null && [[ "$state" != Z* ]]; then
            continue
        fi
        wait "$pid" 2>/dev/null || true
        log "${name} encerrou; reiniciando"
        start_process "$name" "${process_directories[$name]}" "${process_executables[$name]}"
    done
}

trap stop_all EXIT INT TERM
mkdir -p "$log_dir"
validate_config_value MYSQL_ROOT_PASSWORD "$root_password"

if [[ ! -f "$data_dir/.openmetin-legacy-imported" ]]; then
    initialize_legacy_databases
fi

start_database
wait_for_database
ensure_database_user
configure_game

start_process cache-db /usr/game/core/db ./db
wait_for_cache_db
start_process auth /usr/game/core/auth ./auth
for core in 1 2 3 4; do
    start_process "ch1-core${core}" "/usr/game/core/ch1/core${core}" ./game
done
start_process game99 /usr/game/core/game99 ./game

log 'servidor legado iniciado'
while true; do
    supervise_processes
    sleep 2
done
