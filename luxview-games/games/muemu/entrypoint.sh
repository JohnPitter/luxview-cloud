#!/bin/bash
set -e

DATA_DIR="/muemu-data"
GS_DIR="/opt/muemu/gameserver"
CS_DIR="/opt/muemu/connectserver"

MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-muemu}"
MUEMU_SERVER_NAME="${MUEMU_SERVER_NAME:-MU Online Server}"
MUEMU_SEASON="${MUEMU_SEASON:-Season9Eng}"
MUEMU_LANGUAGE="${MUEMU_LANGUAGE:-en}"
MUEMU_AUTO_REGISTER="${MUEMU_AUTO_REGISTER:-true}"
MUEMU_EXP_RATE="${MUEMU_EXP_RATE:-9000}"
MUEMU_DROP_RATE="${MUEMU_DROP_RATE:-60}"
MUEMU_ZEN_RATE="${MUEMU_ZEN_RATE:-10}"
MUEMU_GOLD_EXP="${MUEMU_GOLD_EXP:-0}"
MUEMU_MAX_PARTY_LEVEL_DIFF="${MUEMU_MAX_PARTY_LEVEL_DIFF:-400}"
MUEMU_CLIENT_VERSION="${MUEMU_CLIENT_VERSION:-10525}"
MUEMU_CLIENT_SERIAL="${MUEMU_CLIENT_SERIAL:-fughy683dfu7teqg}"

mkdir -p "$DATA_DIR/mysql"

# Initialize MySQL data directory if first run
if [ ! -d "$DATA_DIR/mysql/mysql" ]; then
    echo "[muemu] Initializing MySQL database..."
    mysqld --initialize-insecure --datadir="$DATA_DIR/mysql" --user=mysql
    chown -R mysql:mysql "$DATA_DIR/mysql"

    # Start MySQL temporarily to set password and create database
    mysqld --datadir="$DATA_DIR/mysql" --user=mysql --skip-networking &
    MYSQL_PID=$!

    # Wait for MySQL to be ready
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

# Generate server.xml for GameServer
cat > "$GS_DIR/server.xml" <<XMLEOF
<?xml version="1.0"?>
<Server xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <Name>${MUEMU_SERVER_NAME}</Name>
  <Code>0</Code>
  <Show>1</Show>
  <Lang>${MUEMU_LANGUAGE}</Lang>
  <AutoRegister>${MUEMU_AUTO_REGISTER}</AutoRegister>
  <Season>${MUEMU_SEASON}</Season>
  <Connection>
    <IP>0.0.0.0</IP>
    <Port>55901</Port>
    <ConnectServerIP>127.0.0.1</ConnectServerIP>
    <APIKey>2020110116</APIKey>
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

echo "[muemu] server.xml generated (Season: ${MUEMU_SEASON}, EXP: ${MUEMU_EXP_RATE}x)"
echo "[muemu] Starting services via supervisord..."
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/muemu.conf
