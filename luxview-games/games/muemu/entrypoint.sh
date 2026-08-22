#!/bin/bash
set -e

DATA_DIR="/muemu-data"
GS_DIR="/opt/muemu/gameserver"
GS99_DIR="/opt/muemu/gameserver99"
CS_DIR="/opt/muemu/connectserver"
API_KEY="2020110116"

MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-muemu}"
MUEMU_SERVER_NAME="${MUEMU_SERVER_NAME:-MU Online Server}"
MUEMU_SEASON="${MUEMU_SEASON:-Classic99}"
MUEMU_LANGUAGE="${MUEMU_LANGUAGE:-en}"
MUEMU_AUTO_REGISTER="${MUEMU_AUTO_REGISTER:-true}"
MUEMU_EXP_RATE="${MUEMU_EXP_RATE:-9000}"
MUEMU_DROP_RATE="${MUEMU_DROP_RATE:-60}"
MUEMU_ZEN_RATE="${MUEMU_ZEN_RATE:-10}"
MUEMU_GOLD_EXP="${MUEMU_GOLD_EXP:-0}"
MUEMU_MAX_PARTY_LEVEL_DIFF="${MUEMU_MAX_PARTY_LEVEL_DIFF:-400}"
MUEMU_CLIENT_VERSION="${MUEMU_CLIENT_VERSION:-10525}"
MUEMU_CLIENT_SERIAL="${MUEMU_CLIENT_SERIAL:-fughy683dfu7teqg}"
PUBLIC_IP="${LUXVIEW_PUBLIC_IP:-127.0.0.1}"

mkdir -p "$DATA_DIR/mysql"

# Initialize MySQL data directory if first run
if [ ! -d "$DATA_DIR/mysql/mysql" ]; then
    echo "[muemu] Initializing MySQL database..."
    mysqld --initialize-insecure --datadir="$DATA_DIR/mysql" --user=mysql
    chown -R mysql:mysql "$DATA_DIR/mysql"

    mysqld --datadir="$DATA_DIR/mysql" --user=mysql --skip-networking &
    MYSQL_PID=$!

    for i in $(seq 1 30); do
        if mysqladmin ping --silent 2>/dev/null; then
            break
        fi
        sleep 1
    done

    mysql -u root <<-EOSQL
        ALTER USER 'root'@'localhost' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}';
        CREATE DATABASE IF NOT EXISTS MuOnline CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
        FLUSH PRIVILEGES;
EOSQL

    kill "$MYSQL_PID"
    wait "$MYSQL_PID" 2>/dev/null || true
    echo "[muemu] MySQL initialized."
else
    chown -R mysql:mysql "$DATA_DIR/mysql"
fi

DUAL=0
GS0_SEASON="$MUEMU_SEASON"
GS99_SEASON=""
case "$MUEMU_SEASON" in
    Classic99)
        DUAL=1
        GS0_SEASON="Season0Kor"
        GS99_SEASON="Season3Kor"
        # 1.02c is the usual hybrid 97D + Season 2 client serial/version.
        if [ "$MUEMU_CLIENT_VERSION" = "10525" ]; then
            MUEMU_CLIENT_VERSION="10203"
        fi
        ;;
    Season0Kor)
        if [ "$MUEMU_CLIENT_VERSION" = "10525" ]; then
            MUEMU_CLIENT_VERSION="09704"
        fi
        ;;
    Season3Kor)
        if [ "$MUEMU_CLIENT_VERSION" = "10525" ]; then
            MUEMU_CLIENT_VERSION="10203"
        fi
        ;;
esac

write_server_xml() {
    local dest_dir="$1"
    local name="$2"
    local code="$3"
    local season="$4"
    local port="$5"
    local xml
    xml=$(cat <<XMLEOF
<?xml version="1.0"?>
<Server xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <Name>${name}</Name>
  <Code>${code}</Code>
  <Show>1</Show>
  <Lang>${MUEMU_LANGUAGE}</Lang>
  <AutoRegister>${MUEMU_AUTO_REGISTER}</AutoRegister>
  <Season>${season}</Season>
  <Connection>
    <IP>0.0.0.0</IP>
    <IPPublic>${PUBLIC_IP}</IPPublic>
    <Port>${port}</Port>
    <ConnectServerIP>127.0.0.1</ConnectServerIP>
    <APIKey>${API_KEY}</APIKey>
  </Connection>
  <Database>
    <DBIp>127.0.0.1</DBIp>
    <DataBase>MuOnline</DataBase>
    <BDUser>root</BDUser>
    <DBPassword>${MYSQL_ROOT_PASSWORD}</DBPassword>
  </Database>
  <Client>
    <Version>${MUEMU_CLIENT_VERSION}</Version>
    <Serial>${MUEMU_CLIENT_SERIAL}</Serial>
    <CashShopVersion>512.2014.124</CashShopVersion>
  </Client>
  <GamePlay>
    <Experience>${MUEMU_EXP_RATE}</Experience>
    <GoldExperience>${MUEMU_GOLD_EXP}</GoldExperience>
    <Zen>${MUEMU_ZEN_RATE}</Zen>
    <DropRate>${MUEMU_DROP_RATE}</DropRate>
    <MaxPartyLevelDifference>${MUEMU_MAX_PARTY_LEVEL_DIFF}</MaxPartyLevelDifference>
  </GamePlay>
  <Files>
    <Monsters>./Data/Monsters/Monster</Monsters>
    <MonsterSetBase>./Data/Monsters/MonsterSetBase</MonsterSetBase>
    <MapServer>./Data/MapServer.xml</MapServer>
  </Files>
</Server>
XMLEOF
)
    # MuEmu loads ./Server.xml (case-sensitive on Linux).
    printf '%s\n' "$xml" > "$dest_dir/Server.xml"
    printf '%s\n' "$xml" > "$dest_dir/server.xml"
}

GS0_NAME="$MUEMU_SERVER_NAME"
if [ "$DUAL" = "1" ]; then
    GS0_NAME="${MUEMU_SERVER_NAME} 97D"
fi
write_server_xml "$GS_DIR" "$GS0_NAME" "0" "$GS0_SEASON" "55901"

if [ "$DUAL" = "1" ]; then
    if [ ! -d "$GS99_DIR" ]; then
        echo "[muemu] cloning GameServer 99 from GameServer 0..."
        cp -a "$GS_DIR" "$GS99_DIR"
    fi
    write_server_xml "$GS99_DIR" "${MUEMU_SERVER_NAME} Season 2+" "99" "$GS99_SEASON" "55919"
    echo "[muemu] dual world: GS0 ${GS0_SEASON} :55901 + GS99 ${GS99_SEASON} :55919 (client ${MUEMU_CLIENT_VERSION})"
else
    echo "[muemu] Server.xml generated (Season: ${GS0_SEASON}, EXP: ${MUEMU_EXP_RATE}x, client ${MUEMU_CLIENT_VERSION})"
fi

cat > "$CS_DIR/configuration.xml" <<XMLEOF
<?xml version="1.0"?>
<ConnectServer xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <apiKey>${API_KEY}</apiKey>
  <IP>0.0.0.0</IP>
  <IPChat>0.0.0.0</IPChat>
  <DataBase>
    <IP>127.0.0.1</IP>
    <Name>MuOnline</Name>
    <User>root</User>
    <Password>${MYSQL_ROOT_PASSWORD}</Password>
  </DataBase>
</ConnectServer>
XMLEOF

GS99_AUTOSTART="false"
if [ "$DUAL" = "1" ]; then
    GS99_AUTOSTART="true"
fi

cat > /tmp/muemu-supervisord.conf <<CONF
[supervisord]
nodaemon=true
logfile=/dev/null
logfile_maxbytes=0
user=root

[program:mysql]
command=/usr/sbin/mysqld --datadir=/muemu-data/mysql --user=mysql
autostart=true
autorestart=true
priority=10
stdout_logfile=/dev/fd/1
stdout_logfile_maxbytes=0
stderr_logfile=/dev/fd/2
stderr_logfile_maxbytes=0

[program:connectserver]
command=dotnet /opt/muemu/connectserver/CSEmu.dll
directory=/opt/muemu/connectserver
autostart=true
autorestart=true
priority=20
startsecs=10
startretries=5
stdout_logfile=/dev/fd/1
stdout_logfile_maxbytes=0
stderr_logfile=/dev/fd/2
stderr_logfile_maxbytes=0

[program:gameserver]
command=dotnet /opt/muemu/gameserver/MuEmu.dll
directory=/opt/muemu/gameserver
autostart=true
autorestart=true
priority=30
startsecs=15
startretries=5
stdout_logfile=/dev/fd/1
stdout_logfile_maxbytes=0
stderr_logfile=/dev/fd/2
stderr_logfile_maxbytes=0

[program:gameserver99]
command=dotnet /opt/muemu/gameserver99/MuEmu.dll
directory=/opt/muemu/gameserver99
autostart=${GS99_AUTOSTART}
autorestart=true
priority=31
startsecs=15
startretries=5
stdout_logfile=/dev/fd/1
stdout_logfile_maxbytes=0
stderr_logfile=/dev/fd/2
stderr_logfile_maxbytes=0
CONF

echo "[muemu] Starting services via supervisord..."
exec /usr/bin/supervisord -c /tmp/muemu-supervisord.conf
