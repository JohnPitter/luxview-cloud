#!/usr/bin/env bash
# Constrói o prefixo win32 sem downloads: DirectX vendorizado + MDAC manual.
set -euo pipefail
export WINEPREFIX="${WINEPREFIX:-/wine}" WINEARCH=win32 WINEDEBUG=-all DISPLAY=:95
S32="$WINEPREFIX/drive_c/windows/system32"
MDAC_SHA256=157ebae46932cb9047b58aa849ac1885e8cbd2f218810cb83e57613b49c679d6
log() { echo "[priston][bake] $*"; }
Xvfb :95 -screen 0 1024x768x24 & xvfb_pid=$!
trap 'kill "$xvfb_pid" 2>/dev/null || true' EXIT
sleep 1
wineboot -u
wineserver -w
for cab in /opt/vendor/*.cab; do cabextract -q -d "$S32" -L -F 'd3dx9*.dll' "$cab"; done
for v in 35 43; do test -s "$S32/d3dx9_$v.dll" || { log "falta d3dx9_$v.dll"; exit 1; }; done
echo "$MDAC_SHA256  /opt/mdac/MDAC_TYP.EXE" | sha256sum -c -
mkdir -p /opt/mdac/base /opt/mdac/x
cabextract --directory=/opt/mdac/base /opt/mdac/MDAC_TYP.EXE >/dev/null 2>&1 || true
test -f /opt/mdac/base/sqloldb.cab || { log 'extração MDAC sem sqloldb.cab'; exit 1; }
for c in sqloldb sqlnet sqlodbc mdacxpak; do mkdir -p "/opt/mdac/x/$c"; (cd "/opt/mdac/x/$c" && cabextract -q "/opt/mdac/base/$c.cab" || true); done
for f in sqloldb/sqloledb.dll mdacxpak/oledb32.dll sqlnet/dbnetlib.dll sqlodbc/sqlsrv32.dll; do test -s "/opt/mdac/x/$f" || { log "MDAC incompleto: $f"; exit 1; }; done
MDAC_SRC=/opt/mdac/x /usr/local/bin/install-mdac-manual.sh
wineserver -w
test -s "$WINEPREFIX/drive_c/Program Files/Common Files/System/OLE DB/sqloledb.dll"
rm -rf /opt/mdac /opt/vendor /root/.cache/winetricks
log 'prefixo Wine pronto.'
