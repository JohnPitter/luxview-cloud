#!/usr/bin/env bash
# Publica Main.exe + MUnique.Client.Library.dll (e opcionalmente outros arquivos)
# no zip global que a engine usa para GET /api/public/games (client_hash) e
# /game-client/{id}/patch. O Main.exe faz LoadLibrary da DLL na mesma pasta —
# ATUALIZAR precisa entregar os dois ou o client não conecta no GS.
#
# A DLL TEM que ser Native AOT win-x64 (~3.1MB, PE machine 0x8664). x86 faz
# LoadLibrary falhar e o client mostra "MUnique.Client.Library.dll missing".
# NÃO use zip -u: ele recusa substituir um entry mais novo (ex.: x86 recente)
# por um x64 mais antigo.
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
DLL_NAME="MUnique.Client.Library.dll"

assert_pe_x64() {
  local f="$1"
  python3 - "$f" <<'PY'
import os, struct, sys
path = sys.argv[1]
with open(path, "rb") as fh:
    header = fh.read(4096)
if len(header) < 0x40 or header[:2] != b"MZ":
    raise SystemExit(f"{path}: not a PE (missing MZ)")
pe = struct.unpack_from("<I", header, 0x3C)[0]
need = pe + 6
if need > len(header):
    with open(path, "rb") as fh:
        header = fh.read(need)
    if len(header) < need:
        raise SystemExit(f"{path}: truncated PE header")
machine = struct.unpack_from("<H", header, pe + 4)[0]
if machine != 0x8664:
    arch = {0x14C: "x86", 0x8664: "x64"}.get(machine, hex(machine))
    raise SystemExit(
        f"{path}: PE machine is {arch} (0x{machine:x}), need x64 (0x8664) — "
        "x86 MUnique.Client.Library.dll makes LoadLibrary fail as 'missing'"
    )
print(f"{path}: PE x64 OK ({os.path.getsize(path)} bytes)")
PY
}

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
assert_pe_x64 "$WORK/main.exe"

MAIN_DIR=$(dirname "$SRC_MAIN")
if [[ ! -f "$MAIN_DIR/$DLL_NAME" ]]; then
  echo "missing $DLL_NAME next to $SRC_MAIN — Main.exe cannot connect without it" >&2
  exit 1
fi
cp -f "$MAIN_DIR/$DLL_NAME" "$WORK/$DLL_NAME"
assert_pe_x64 "$WORK/$DLL_NAME"
DLL_SIZE=$(stat -c%s "$WORK/$DLL_NAME")
if (( DLL_SIZE < 3000000 )); then
  echo "$DLL_NAME is ${DLL_SIZE} bytes — expected Native AOT win-x64 (~3.1MB), not x86 (~2.6MB)" >&2
  exit 1
fi

CLIENT_LIB_FILES=("$DLL_NAME")
for dep in \
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
  if [[ "$base" == "$DLL_NAME" ]]; then
    continue
  fi
  cp -f "$extra_dll" "$WORK/$base"
  CLIENT_LIB_FILES+=("$base")
done
shopt -u nullglob
echo "client library: ${CLIENT_LIB_FILES[*]}"

for rel in "${EXTRA_PATHS[@]}"; do
  if [[ ! -f "$rel" ]]; then
    echo "extra file not found: $rel" >&2
    exit 1
  fi
  mkdir -p "$WORK/$(dirname "$rel")"
  cp -f "$rel" "$WORK/$rel"
done

# Force-replace. zip -u skips older sources and previously left an x86 DLL in the zip.
echo "=== update zip (force replace, not zip -u) ==="
(
  cd "$WORK"
  zip -q "$ZIP" main.exe
  for dep in "${CLIENT_LIB_FILES[@]}"; do
    zip -q "$ZIP" "$dep"
  done
  for rel in "${EXTRA_PATHS[@]}"; do
    zip -q "$ZIP" "$rel"
  done
)
chmod 644 "$ZIP"
# FileHash da engine usa size+mtime do arquivo — garante bump mesmo se zip preservar mtime.
touch "$ZIP"
sleep 1

VERIFY=/tmp/mu-client-verify-$$
mkdir -p "$VERIFY"
if ! unzip -p "$ZIP" "$DLL_NAME" > "$VERIFY/$DLL_NAME"; then
  echo "LIVE zip $ZIP is missing $DLL_NAME after publish" >&2
  exit 1
fi
assert_pe_x64 "$VERIFY/$DLL_NAME"
VERIFY_SIZE=$(stat -c%s "$VERIFY/$DLL_NAME")
if (( VERIFY_SIZE < 3000000 )); then
  echo "LIVE zip $DLL_NAME is ${VERIFY_SIZE} bytes after publish — not x64 AOT" >&2
  exit 1
fi
echo "LIVE zip $DLL_NAME: ${VERIFY_SIZE} bytes x64 AOT OK"
unzip -l "$ZIP" | grep -iE "MUnique.Client.Library.dll|main.exe" || true
rm -rf "$VERIFY"

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
