#!/bin/sh
# LuxView Priston Tale 4220 — Wine32 + SunnyBPT_v4220.exe
set -eu

server_root="${PRISTON_SERVER_ROOT:-/server}"
public_ip="${LUXVIEW_PUBLIC_IP:-${PRISTON_SERVER_IP:-127.0.0.1}}"
server_name="${PRISTON_SERVER_NAME:-LuxView}"
mssql_host="${PRISTON_MSSQL_HOST:-luxview-mssql}"
mssql_password="${PRISTON_MSSQL_PASSWORD:-}"
export WINEARCH="${WINEARCH:-win32}"
export WINEPREFIX="${WINEPREFIX:-/wine}"
export WINEDEBUG="${WINEDEBUG:--all}"
export WINEDLLOVERRIDES="${WINEDLLOVERRIDES:-mscoree,mshtml=}"
export PRISTON_MSSQL_HOST="$mssql_host"
export PRISTON_MSSQL_PASSWORD="$mssql_password"
export PATH="/opt/mssql-tools18/bin:${PATH}"

mkdir -p /data/state /artifacts "$server_root/LogFile" "$server_root/ptLog"

if [ ! -x "$server_root/SunnyBPT_v4220.exe" ] && [ ! -f "$server_root/SunnyBPT_v4220.exe" ]; then
  echo "[priston] SunnyBPT_v4220.exe ausente em $server_root" >&2
  ls -la "$server_root" >&2 || true
  exit 1
fi

hotuk="$server_root/hotuk.ini"
if [ ! -f "$hotuk" ]; then
  echo "[priston] hotuk.ini ausente" >&2
  exit 1
fi

bind_ip="$(hostname -i 2>/dev/null | awk '{print $1}')"
if [ -z "$bind_ip" ] || [ "$bind_ip" = "127.0.0.1" ]; then
  bind_ip="0.0.0.0"
fi

# Native 4220: keep official GameServer NPC/mob/field data. Only rewrite
# listen/advertise + server name. Rates stay commented unless already present.
tmp_hotuk="$(mktemp)"
awk -v name="$server_name" -v bind="$bind_ip" -v pub="$public_ip" '
  BEGIN { IGNORECASE=1 }
  /^\*SERVER_NAME/ { print "*SERVER_NAME\t\t" name; next }
  /^\*GAME_SERVER/ { print "*GAME_SERVER\t\tSunnyBPT_v4220\t" bind "\t" pub "\t" pub; next }
  /^\*SYSTEM_IP/ { print "*SYSTEM_IP  " pub " " pub; next }
  { print }
' "$hotuk" > "$tmp_hotuk"
cp "$tmp_hotuk" "$hotuk"
rm -f "$tmp_hotuk"

if [ -f /opt/patch_sql_dll.py ]; then
  src="$server_root/sql.dll.original"
  if [ ! -f "$src" ]; then
    src="$server_root/sql.dll"
  fi
  python3 /opt/patch_sql_dll.py --src "$src" --dst "$server_root/sql.dll" --server "${mssql_host},1433" || \
    echo "[priston] aviso: não consegui retargetar sql.dll para ${mssql_host},1433" >&2
fi

for dll in msvcr70.dll mfc70.dll sql.dll PristonSQLDll.dll clan.dll clan-procedure.dll; do
  if [ -f "$server_root/$dll" ]; then
    cp -f "$server_root/$dll" "$WINEPREFIX/drive_c/windows/system32/$dll" 2>/dev/null || true
  fi
done

if [ -n "$mssql_password" ]; then
  i=0
  while [ "$i" -lt 30 ]; do
    if /opt/mssql-tools18/bin/sqlcmd -S "$mssql_host" -U sa -P "$mssql_password" -C -Q "SELECT 1" >/dev/null 2>&1; then
      /opt/mssql-tools18/bin/sqlcmd -S "$mssql_host" -U sa -P "$mssql_password" -C -i /opt/init-accountdb.sql || true
      break
    fi
    i=$((i + 1))
    sleep 2
  done
fi

python3 /opt/priston-account.py serve &

echo "[priston] LuxView $server_name native 4220 bind=${bind_ip} advertise=${public_ip}:10012 mssql=${mssql_host}"
cd "$server_root"
# -ac disables X authority so Wine can create the (headless) server window.
rm -f /tmp/.X99-lock
Xvfb :99 -ac -screen 0 1024x768x16 -nolisten tcp >/artifacts/xvfb.log 2>&1 &
export DISPLAY=:99
sleep 1
exec wine "$server_root/SunnyBPT_v4220.exe"
