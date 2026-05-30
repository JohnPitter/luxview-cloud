#!/bin/bash
# Rakion (SoftNyx v258) — entrypoint para LuxView Cloud.
# Sobe: MariaDB (persistido em /var/lib/mysql) + auth web PHP + BrokenServer + RakionWorldServ.
set -u
log() { echo "[rakion] $*"; }

SRV=/server
BROKER="$SRV/BrokenServer"
WORLD="$SRV/RakionWorldServ"
WEB=/webroot

# --- Config via env (template LuxView) ---------------------------------------
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-123456}"
RAKION_ADMIN_PASS="${RAKION_ADMIN_PASS:-admin123}"

#############################################
# 1. MariaDB (datadir = /var/lib/mysql, volume persistente)
#############################################
log "iniciando MariaDB..."
mkdir -p /run/mysqld && chown -R mysql:mysql /run/mysqld
FIRST_INIT=0
if [ ! -d /var/lib/mysql/mysql ]; then
    FIRST_INIT=1
    log "primeira inicialização — instalando datadir do MariaDB..."
    mariadb-install-db --user=mysql --datadir=/var/lib/mysql >/dev/null 2>&1
fi
chown -R mysql:mysql /var/lib/mysql
mariadbd --user=mysql --bind-address=0.0.0.0 --lower-case-table-names=1 >/var/log/mariadb.log 2>&1 &
for i in $(seq 1 60); do mysqladmin ping --silent 2>/dev/null && break; sleep 1; done
mysqladmin ping 2>/dev/null && log "MariaDB up" || { log "MariaDB FALHOU"; cat /var/log/mariadb.log; }

# Senha do root + acesso (idempotente)
mysql 2>/dev/null <<SQL || mysql -uroot -p"$MYSQL_ROOT_PASSWORD" 2>/dev/null <<SQL2
SET PASSWORD FOR 'root'@'localhost' = PASSWORD('${MYSQL_ROOT_PASSWORD}');
CREATE USER IF NOT EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}';
CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'127.0.0.1' WITH GRANT OPTION;
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;
SQL
SET PASSWORD FOR 'root'@'localhost' = PASSWORD('${MYSQL_ROOT_PASSWORD}');
FLUSH PRIVILEGES;
SQL2

MQ() { mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$@"; }

# Carrega o DUMP só na PRIMEIRA inicialização (depois, contas persistem no volume)
HAS_DB=$(MQ -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='rakion';" 2>/dev/null || echo 0)
if [ "$FIRST_INIT" = "1" ] || [ "${HAS_DB:-0}" = "0" ]; then
    if [ -f "$SRV/DB/rakion_data.sql" ]; then
        log "carregando rakion_data.sql (dump funcional)..."
        MQ < "$SRV/DB/rakion_data.sql" 2>&1 | sed 's/^/[db] /' | tail -3
    elif [ -f "$SRV/DB/rakion_all.sql" ]; then
        log "carregando rakion_all.sql (schema)..."
        MQ -e "CREATE DATABASE IF NOT EXISTS rakion CHARACTER SET utf8;"
        MQ rakion < "$SRV/DB/rakion_all.sql" 2>&1 | sed 's/^/[db] /' | tail -3
    fi
    # Conta de teste + fetchapp (idempotente)
    MQ rakion <<'SQL' 2>&1 | sed 's/^/[acct] /'
INSERT IGNORE INTO user (id,password) VALUES ('test','test');
INSERT IGNORE INTO usergameinfo (id) VALUES (1);
UPDATE usergameinfo SET gold=GREATEST(gold,10000) WHERE id=1;
INSERT IGNORE INTO fetchapp (AppId,VerLimit) VALUES (400,1),(11001,258);
SQL
else
    log "banco rakion já existe (volume persistente) — preservando contas."
fi
TBL=$(MQ -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='rakion';" 2>/dev/null)
log "tabelas rakion: ${TBL:-?}"

#############################################
# 2. Aplica config nos PHP (senha DB + senha admin) e patcha IPs do servidor
#############################################
log "aplicando senhas nos configs PHP..."
# DB password nos três configs (config.php, admin, fetch)
sed -i "s/define('MYSQL_PASS', \"[^\"]*\")/define('MYSQL_PASS', \"${MYSQL_ROOT_PASSWORD}\")/" "$WEB/config.php" 2>/dev/null || true
sed -i "s/define('DB_PASS', '[^']*')/define('DB_PASS', '${MYSQL_ROOT_PASSWORD}')/" "$WEB/admin/config_admin.php" 2>/dev/null || true
sed -i "s/\$config\['db_pass'\] = '[^']*'/\$config['db_pass'] = '${MYSQL_ROOT_PASSWORD}'/" "$WEB/fetch/fetch.php" 2>/dev/null || true
# Senha do painel admin
sed -i "s/define('ADMIN_PASS', '[^']*')/define('ADMIN_PASS', '${RAKION_ADMIN_PASS}')/" "$WEB/admin/config_admin.php" 2>/dev/null || true

log "ajustando IPs nos *.ini do servidor (192.168.1.x -> 127.0.0.1)..."
find "$SRV" -name '*.ini' -print0 2>/dev/null | while IFS= read -r -d '' f; do
    sed -i 's/192\.168\.1\.[0-9]\+/127.0.0.1/g' "$f"
done
log "csauth2.cfg = $(cat "$WORLD/csauth2.cfg" 2>/dev/null) (GameGuard server-side mantido)"

#############################################
# 3. Auth web PHP em :80 (roteado pelo Traefik via subdomínio, HTTP puro)
#############################################
log "subindo auth web (php -S :80 -t $WEB)..."
php -S 0.0.0.0:80 -t "$WEB" >/var/log/php.log 2>&1 &

#############################################
# 4. Xvfb + Wine (broker + world). Sequência validada (e2e_start.sh).
#############################################
export DISPLAY=:0 WINEARCH=win32
unset WINEDEBUG WINEDLLOVERRIDES
Xvfb :0 -screen 0 1024x768x16 >/var/log/xvfb.log 2>&1 &
sleep 2

log "=== BrokenServer.exe (broker :40706) ==="
cd "$BROKER" || exit 1
wine BrokenServer.exe >"$SRV/broker_run.log" 2>&1 &
sleep 10

log "=== RakionWorldServ.exe (world :40708) — install + SCM start ==="
cd "$WORLD" || exit 1
wine RakionWorldServ.exe -install >"$SRV/world_install.log" 2>&1
sleep 3
wine sc start "Rakion World [1]" >"$SRV/world_run.log" 2>&1
# o world sob Wine leva ~30-90s pra bindar; aguarda até 120s antes do fallback
for i in $(seq 1 24); do ss -ltn 2>/dev/null | grep -q ':40708' && break; sleep 5; done
if ! ss -ltn 2>/dev/null | grep -q ':40708'; then
    log "world via SCM não bindou em 120s; tentando launch direto..."
    wine RakionWorldServ.exe >>"$SRV/world_run.log" 2>&1 &
    sleep 15
fi
ss -ltn 2>/dev/null | grep -q ':40708' && log "world OK (:40708 bound)" || log "world AINDA não bindou — ver world_run.log"

#############################################
# 5. Status + mantém o container vivo
#############################################
echo; log "######## PORTAS ########"
ss -ltnp 2>/dev/null | grep -E '4070[0-9]|:80 |3306' || log "(nenhuma porta rakion)"
echo; log "servidor de pé — segurando aberto (tail dos logs)"
tail -f "$SRV/broker_run.log" "$SRV/world_run.log" /var/log/php.log
