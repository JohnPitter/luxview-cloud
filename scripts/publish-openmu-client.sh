#!/usr/bin/env bash
# Publica Main.exe + MUnique.Client.Library.dll (e opcionalmente outros arquivos)
# no zip global que a engine usa para GET /api/public/games (client_hash) e
# /game-client/{id}/patch. O Main.exe faz LoadLibrary da DLL na mesma pasta —
# ATUALIZAR precisa entregar os dois ou o client não conecta no GS.
#
# Caminho correto (NÃO use storage/app-<id>/ — isso não altera o catálogo):
#   /data/luxview/storage/_global/openmu-assets/openmu-s6-base.zip
#
# Uso na VPS:
#   bash scripts/publish-openmu-client.sh
#   bash scripts/publish-openmu-client.sh /caminho/main.exe
#   bash scripts/publish-openmu-client.sh /caminho/main.exe Data/World1/EncTerrain1.att
set -euo pipefail

ZIP="/data/luxview/storage/_global/openmu-assets/openmu-s6-base.zip"
SRC_MAIN="${1:-}"
shift || true
EXTRA_PATHS=("$@")

if [[ -z "$SRC_MAIN" ]]; then
  for c in /data/luxview/mu-web/patch/main.exe /tmp/main.exe.new /tmp/publish-main.exe; do
    if [[ -f "$c" ]]; then SRC_MAIN="$c"; break; fi
  done
fi
if [[ -z "$SRC_MAIN" || ! -f "$SRC_MAIN" ]]; then
  echo "missing Main.exe source (arg or /data/luxview/mu-web/patch/main.exe)" >&2
  exit 1
fi
if [[ ! -f "$ZIP" ]]; then
  echo "missing client zip $ZIP" >&2
  exit 1
fi

STAMP=$(date -u +%Y%m%d%H%M%S)
BACKUP="${ZIP}.bak-publish-${STAMP}"
cp -f "$ZIP" "$BACKUP"

WORK=/tmp/mu-client-publish-$$
mkdir -p "$WORK"
cp -f "$SRC_MAIN" "$WORK/main.exe"

MAIN_DIR=$(dirname "$SRC_MAIN")
CLIENT_LIB_FILES=()
for dep in \
  "MUnique.Client.Library.dll" \
  "MUnique.Client.Library.runtimeconfig.json" \
  "hostfxr.dll"
do
  if [[ -f "$MAIN_DIR/$dep" ]]; then
    cp -f "$MAIN_DIR/$dep" "$WORK/$dep"
    CLIENT_LIB_FILES+=("$dep")
  fi
done
shopt -s nullglob
for extra_dll in "$MAIN_DIR"/MUnique.Client.*.dll; do
  base=$(basename "$extra_dll")
  if [[ "$base" == "MUnique.Client.Library.dll" ]]; then
    continue
  fi
  cp -f "$extra_dll" "$WORK/$base"
  CLIENT_LIB_FILES+=("$base")
done
shopt -u nullglob
if [[ ${#CLIENT_LIB_FILES[@]} -eq 0 ]]; then
  echo "missing MUnique.Client.Library.dll next to $SRC_MAIN — Main.exe cannot connect without it" >&2
  exit 1
fi
echo "client library: ${CLIENT_LIB_FILES[*]}"

for rel in "${EXTRA_PATHS[@]}"; do
  if [[ ! -f "$rel" ]]; then
    echo "extra file not found: $rel" >&2
    exit 1
  fi
  mkdir -p "$WORK/$(dirname "$rel")"
  cp -f "$rel" "$WORK/$rel"
done

echo "=== update zip (zip -u) ==="
(
  cd "$WORK"
  zip -q -u "$ZIP" main.exe
  for dep in "${CLIENT_LIB_FILES[@]}"; do
    zip -q -u "$ZIP" "$dep"
  done
  for rel in "${EXTRA_PATHS[@]}"; do
    zip -q -u "$ZIP" "$rel"
  done
)
chmod 644 "$ZIP"
# FileHash da engine usa size+mtime do arquivo — garante bump mesmo se zip -u preservar mtime.
touch "$ZIP"
sleep 1

python3 - <<'PY'
import hashlib, json, os, time, urllib.request

zip_path = "/data/luxview/storage/_global/openmu-assets/openmu-s6-base.zip"
st = os.stat(zip_path)
key = f"{zip_path}|{st.st_size}|{st.st_mtime_ns}"
base_hash = hashlib.sha256(key.encode()).hexdigest()
print(f"zip size={st.st_size} mtime={time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(st.st_mtime))}")
print(f"computed base_hash={base_hash}")

data = json.load(urllib.request.urlopen("https://luxview.cloud/api/public/games"))
mu = next(x for x in data if x.get("game") == "openmu")
print(f"API base_hash={mu['base_hash']}")
print(f"API client_hash={mu['client_hash']}")
if mu["base_hash"] != base_hash:
    raise SystemExit("catalog base_hash stale — engine not reading the global zip?")
print("catalog OK — players with older client_hash should see ATUALIZAR within ~60s")
PY

echo "backup=$BACKUP"
echo "PUBLISH_OK"
