#!/bin/bash
# Prepara o wineprefix definitivo: wineboot, d3dx9_35/43 nativos (cabs
# vendored) e MDAC 2.8 SP1 manual — nada é baixado durante o build.
set -euo pipefail
export WINEPREFIX="${WINEPREFIX:-/wine}" WINEARCH=win32 WINEDEBUG=-all \
       WINEDLLOVERRIDES=mscoree,mshtml= DISPLAY=:95

MDAC_SHA256="157ebae46932cb9047b58aa849ac1885e8cbd2f218810cb83e57613b49c679d6"
S32="$WINEPREFIX/drive_c/windows/system32"

Xvfb :95 -screen 0 1024x768x16 &
trap 'kill %1 2>/dev/null || true' EXIT
sleep 1

wineboot -u
wineserver -w

# d3dx9_35/43 nativos (únicos importados pelo SunnyBPT) a partir dos cabs
for cab in /opt/vendor/*.cab; do
  cabextract -q -d "$S32" -L -F 'd3dx9*.dll' "$cab"
done
for v in 35 43; do
  test -s "$S32/d3dx9_$v.dll" || { echo "[bake] falta d3dx9_$v.dll"; exit 1; }
done

# MDAC 2.8 SP1: o instalador oficial falha (status 43) no wine 9; layout manual
echo "${MDAC_SHA256}  /opt/mdac/MDAC_TYP.EXE" | sha256sum -c -
mkdir -p /opt/mdac/base
cabextract --directory=/opt/mdac/base /opt/mdac/MDAC_TYP.EXE >/dev/null 2>&1 || true
test -f /opt/mdac/base/sqloldb.cab || { echo "[bake] extração base sem sqloldb.cab"; exit 1; }

# cabextract devolve exit 1 em warnings não fatais; validar por conteúdo.
X=/opt/mdac/x
for c in sqloldb sqlnet sqlodbc mdacxpak; do
  mkdir -p "$X/$c"
  (cd "$X/$c" && cabextract -q "/opt/mdac/base/$c.cab" || true)
done
test -f "$X/sqloldb/sqloledb.dll"   || { echo "[bake] subcab sqloldb vazio"; exit 1; }
test -f "$X/mdacxpak/oledb32.dll"   || { echo "[bake] subcab mdacxpak vazio"; exit 1; }
test -f "$X/sqlnet/dbnetlib.dll"    || { echo "[bake] subcab sqlnet vazio"; exit 1; }
test -f "$X/sqlodbc/sqlsrv32.dll"   || { echo "[bake] subcab sqlodbc vazio"; exit 1; }

MDAC_SRC="$X" /usr/local/bin/install-mdac-manual.sh
wineserver -w

# validação final: provider no lugar oficial e registrado
OLEDB="$WINEPREFIX/drive_c/Program Files/Common Files/System/OLE DB"
test -s "$OLEDB/sqloledb.dll" || { echo "[bake] FALTA sqloledb.dll"; exit 1; }
test -s "$OLEDB/oledb32.dll"  || { echo "[bake] FALTA oledb32.dll"; exit 1; }
CLSID_PATH=$(wine reg query 'HKCR\\CLSID\\{0C7FF16C-38E3-11d0-97AB-00C04FC2AD98}\\InprocServer32' 2>/dev/null | grep REG_SZ | sed 's/.*REG_SZ *//;s/\r//')
echo "[bake] CLSID InprocServer32 = $CLSID_PATH"
case "$CLSID_PATH" in
  *"System\\OLE DB\\sqloledb.dll"*) ;;
  *) echo "[bake] registro do SQLOLEDB incorreto"; exit 1 ;;
esac

rm -rf /opt/mdac /opt/vendor /root/.cache/winetricks
find "$WINEPREFIX" -name '*.tmp' -delete || true
echo "[bake] prefixo pronto."