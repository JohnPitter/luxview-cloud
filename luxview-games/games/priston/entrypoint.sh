#!/usr/bin/env bash
# Runtime LuxView Priston 4220, portado do legacy-docker.
set -euo pipefail
log() { echo "[priston] $(date -u +%FT%TZ) $*"; }
server_root="${PRISTON_SERVER_ROOT:-/server}"
public_ip="${LUXVIEW_PUBLIC_IP:-${PRISTON_PUBLIC_IP:-127.0.0.1}}"
server_name="${PRISTON_SERVER_NAME:-LuxView}"
# The legacy hotuk parser treats GAME_SERVER fields as TAB-delimited tokens;
# keep the advertised executable/name token space-free to avoid shifting IP fields.
game_server_name="${PRISTON_GAME_SERVER_NAME:-LuxView-Priston}"
sql_host="${PRISTON_MSSQL_HOST:-luxview-mssql}"
sql_port="${PRISTON_MSSQL_PORT:-1433}"
sql_password="${PRISTON_MSSQL_PASSWORD:-}"
export WINEARCH="${WINEARCH:-win32}" WINEPREFIX="${WINEPREFIX:-/wine}" WINEDEBUG="${WINEDEBUG:--all}"
export WINEDLLOVERRIDES="${WINEDLLOVERRIDES:-mscoree,mshtml=;msvcr70,mfc70=n;msado15,oledb32,msdasql,msdaps,msdaenum,msdatl3=n;odbc32,odbccp32,odbcint=b;d3dx9_43,d3dx9_35=n;d3d9=b}"
export DISPLAY="${DISPLAY:-:99}" LIBGL_ALWAYS_SOFTWARE="${LIBGL_ALWAYS_SOFTWARE:-1}"
export PRISTON_MSSQL_HOST="$sql_host" PRISTON_MSSQL_PORT="$sql_port" PRISTON_MSSQL_PASSWORD="$sql_password"
export PATH="/opt/mssql-tools18/bin:$PATH"
mkdir -p /data/state /artifacts "$server_root/LogFile" "$server_root/ptLog" "$WINEPREFIX/drive_c/windows/temp"
source_exe="$server_root/SunnyBPT_v4220.exe"; patched_exe="$server_root/SunnyBPT_docker.exe"
test -f "$source_exe" || { log "ERRO: $source_exe ausente" >&2; exit 1; }

# LAA deixa o processo usar o espaço necessário para os dados do servidor.
if [ ! -f "$patched_exe" ] || [ "$source_exe" -nt "$patched_exe" ]; then
  python3 - "$source_exe" "$patched_exe" <<'PY'
import sys
src, dst = sys.argv[1:]
data = bytearray(open(src, 'rb').read())
if data[:2] != b'MZ': raise SystemExit('PE inválido: assinatura MZ ausente')
pe = int.from_bytes(data[0x3c:0x40], 'little')
if data[pe:pe+4] != b'PE\0\0': raise SystemExit('PE inválido: assinatura PE ausente')
off = pe + 0x16
chars = int.from_bytes(data[off:off+2], 'little')
data[off:off+2] = (chars | 0x20).to_bytes(2, 'little')
open(dst, 'wb').write(data)
PY
  log 'SunnyBPT_docker.exe LARGEADDRESSAWARE gerado (original preservado)'
fi
chmod +x "$patched_exe" 2>/dev/null || true

hotuk="$server_root/hotuk.ini"
test -f "$hotuk" || { log 'ERRO: hotuk.ini ausente' >&2; exit 1; }
tmp_hotuk=$(mktemp); awk -v name="$server_name" -v game="$game_server_name" -v pub="$public_ip" 'BEGIN{IGNORECASE=1} /^\*SERVER_NAME/{print "*SERVER_NAME\t\t"name;next} /^\*GAME_SERVER/{print "*GAME_SERVER\t\t"game"\t"pub"\t"pub"\t"pub;next} /^\*SYSTEM_IP/{print "*SYSTEM_IP  "pub" "pub;next} {print}' "$hotuk" > "$tmp_hotuk"; cp "$tmp_hotuk" "$hotuk"; rm -f "$tmp_hotuk"

# Relay obrigatório: sql.dll tem Data Source=127.0.0.1,1433 hardcoded.
for i in $(seq 1 60); do (echo >/dev/tcp/"$sql_host"/"$sql_port") 2>/dev/null && break; [ "$i" -eq 60 ] && { log "ERRO: MSSQL inacessível em $sql_host:$sql_port"; exit 1; }; sleep 1; done
socat "TCP-LISTEN:1433,bind=127.0.0.1,fork,reuseaddr" "TCP:$sql_host:$sql_port" & socat_pid=$!
trap 'kill "$socat_pid" 2>/dev/null || true' EXIT
log "relay 127.0.0.1:1433 -> $sql_host:$sql_port"

# O patch é somente fallback; por padrão, o sql.dll original e o relay são usados.
# Quando habilitado, preserva uma cópia .orig e não reaplica sobre um DLL já patchado.
patch_sql_dll="${PRISTON_PATCH_SQL_DLL:-0}"
case "${patch_sql_dll,,}" in
  1|true|yes|on)
    if [ -f /opt/patch_sql_dll.py ]; then
      dll="$server_root/sql.dll"
      marker="$server_root/sql.dll.patched"
      src="$server_root/sql.dll.original"
      [ -f "$src" ] || src="$server_root/sql.dll.orig"
      if [ ! -f "$src" ]; then
        src="$dll"
        [ -f "$src" ] && cp -p "$src" "$server_root/sql.dll.orig"
        src="$server_root/sql.dll.orig"
      fi
      if [ -f "$marker" ]; then
        log 'sql.dll patch já aplicado; mantendo backup .orig'
      elif [ -f "$src" ]; then
        if python3 /opt/patch_sql_dll.py --src "$src" --dst "$dll" --server "127.0.0.1,1433"; then
          printf 'patched-from=%s\\n' "$src" > "$marker"
          log 'sql.dll patch opcional aplicado (backup .orig preservado)'
        else
          log 'aviso: patch opcional falhou'
        fi
      else
        log 'aviso: sql.dll ausente; patch opcional ignorado'
      fi
    else
      log 'aviso: /opt/patch_sql_dll.py ausente; patch opcional ignorado'
    fi
    ;;
  *) log 'sql.dll original mantido; relay socat ativo' ;;
esac
for dll in msvcr70.dll mfc70.dll sql.dll PristonSQLDll.dll clan.dll clan-procedure.dll d3dx9_35.dll D3DX9_43.dll; do [ -f "$server_root/$dll" ] && cp -f "$server_root/$dll" "$WINEPREFIX/drive_c/windows/system32/$dll" 2>/dev/null || true; done

regfile="$WINEPREFIX/drive_c/windows/temp/pt-game.reg"
cat > "$regfile" <<EOF
REGEDIT4

[HKEY_LOCAL_MACHINE\Software\ODBC\ODBCINST.INI\SQL Server]
"Driver"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"Setup"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"APILevel"="2"
"ConnectFunctions"="YYN"
"DriverODBCVer"="03.52"

[HKEY_LOCAL_MACHINE\Software\ODBC\ODBCINST.INI\ODBC Drivers]
"SQL Server"="Installed"

[HKEY_LOCAL_MACHINE\Software\ODBC\ODBC.INI\m2master]
"Driver"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"Server"="127.0.0.1"
"Database"="accountdb"

[HKEY_LOCAL_MACHINE\Software\ODBC\ODBC.INI\ODBC Data Sources]
"m2master"="SQL Server"

[HKEY_LOCAL_MACHINE\Software\OpenPriston\SqlAdapter]
"SqlUser"="sa"
"SqlPassword"="$sql_password"

[HKEY_LOCAL_MACHINE\Software\PristonTale\GameServer]
"ServerName"="$server_name"
"server1"="127.0.0.1,1433"
"LogPath"="Z:\\server\\LogFile"
"AccountDbIP"="127.0.0.1,1433"
"AccountDbID"="sa"
"AccountDbPwd"="$sql_password"
"AccountDbName"="accountdb"
"BillingDbIP"="127.0.0.1,1433"
"BillingDbID"="sa"
"BillingDbPwd"="$sql_password"
"BillingDbName"="BillingDb"
"BillingLogDbIP"="127.0.0.1,1433"
"BillingLogDbID"="sa"
"BillingLogDbPwd"="$sql_password"
"BillingLogDbName"="BillingLogDb"
"LogDbIP"="127.0.0.1,1433"
"LogDbID"="sa"
"LogDbPwd"="$sql_password"
"LogDbName"="GameLogDb"
"PCDbIP"="127.0.0.1,1433"
"PCDbID"="sa"
"PCDbPwd"="$sql_password"
"PCDbName"="PCRoom"
"PCCheck"="0"
"PCLogDbIP"="127.0.0.1,1433"
"PCLogDbID"="sa"
"PCLogDbPwd"="$sql_password"
"PCLogDbName"="PCRoomLog"
"ITEMLogDbIP"="127.0.0.1,1433"
"ITEMLogDbID"="sa"
"ITEMLogDbPwd"="$sql_password"
"ITEMLogDbName"="ItemLogDb"
"ClanDbIP"="127.0.0.1,1433"
"ClanDbID"="sa"
"ClanDbPwd"="$sql_password"
"ClanDbName"="ClanDb"
"SODDbIP"="127.0.0.1,1433"
"SODDbID"="sa"
"SODDbPwd"="$sql_password"
"SODDbName"="Sod2Db"
EOF
wine regedit /S 'C:\windows\temp\pt-game.reg'
# regedit can skip ODBC sections on some Wine builds; enforce adapter/DSN keys.
wine reg add 'HKLM\Software\OpenPriston\SqlAdapter' /v SqlUser /d sa /f >/dev/null
wine reg add 'HKLM\Software\OpenPriston\SqlAdapter' /v SqlPassword /d "$sql_password" /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBCINST.INI\SQL Server' /v Driver /d 'C:\windows\system32\sqlsrv32.dll' /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBCINST.INI\SQL Server' /v Setup /d 'C:\windows\system32\sqlsrv32.dll' /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBCINST.INI\ODBC Drivers' /v 'SQL Server' /d Installed /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBC.INI\m2master' /v Driver /d 'C:\windows\system32\sqlsrv32.dll' /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBC.INI\m2master' /v Server /d 127.0.0.1 /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBC.INI\m2master' /v Database /d accountdb /f >/dev/null
wine reg add 'HKLM\Software\ODBC\ODBC Data Sources' /v m2master /d 'SQL Server' /f >/dev/null
for reg_value in AccountDbName BillingDbName BillingLogDbName LogDbName PCDbName PCLogDbName ITEMLogDbName ClanDbName SODDbName; do
  wine reg query 'HKLM\Software\PristonTale\GameServer' /v "$reg_value" >/dev/null || { log "ERRO: registro crítico ausente: $reg_value"; exit 1; }
done
log 'registro GameServer completo configurado.'

if [ -n "$sql_password" ]; then
  for i in $(seq 1 30); do /opt/mssql-tools18/bin/sqlcmd -S "127.0.0.1,1433" -U sa -P "$sql_password" -C -b -Q 'SELECT 1' >/dev/null 2>&1 && break; [ "$i" -eq 30 ] && log 'aviso: MSSQL não respondeu'; sleep 2; done
  if /opt/mssql-tools18/bin/sqlcmd -S "127.0.0.1,1433" -U sa -P "$sql_password" -C -b -i /opt/init-accountdb.sql >/artifacts/schema.log 2>&1; then log 'schema accountdb e bancos satélite aplicado.'; else log 'aviso: schema não aplicado; continuando boot (veja /artifacts/schema.log)'; fi
else log 'aviso: PRISTON_MSSQL_PASSWORD ausente; schema não aplicado'; fi
python3 /opt/priston-account.py serve &
rm -f /tmp/.X99-lock
Xvfb :99 -ac -screen 0 1024x768x24 -nolisten tcp >/artifacts/xvfb.log 2>&1 &
sleep 1
cd "$server_root"
log "iniciando $patched_exe; advertise=${public_ip}:10012"
exec wine "$patched_exe" >/artifacts/wine.log 2>&1
