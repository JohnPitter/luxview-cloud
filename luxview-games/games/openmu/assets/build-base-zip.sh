#!/usr/bin/env bash
# build-base-zip.sh — monta o openmu-s6-base.zip que a engine usa para gerar
# os clients de download de cada servidor OpenMU.
#
# Automatiza tudo que é automatizável: baixa o OpenMU ClientLauncher oficial,
# junta com a pasta do client MU Season 6 que você fornecer e empacota o zip
# no path montado pela engine.
#
# Uso:
#   ./build-base-zip.sh <pasta-do-client-mu-s6>
#
# A <pasta-do-client-mu-s6> precisa conter main.exe + os data folders do
# client Season 6 Episode 3 (English). Esse client é copyright da Webzen e
# NÃO tem download livre — pegue na FAQ do Discord do OpenMU:
#   https://discord.gg/2u5Agkd
set -euo pipefail

LAUNCHER_URL="https://github.com/MUnique/OpenMU/releases/download/v0.9.0/MUnique.OpenMU.ClientLauncher_0.9.6.zip"
ASSETS_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT_ZIP="$ASSETS_DIR/openmu-s6-base.zip"

CLIENT_DIR="${1:-}"
if [[ -z "$CLIENT_DIR" || ! -d "$CLIENT_DIR" ]]; then
  echo "Uso: $0 <pasta-do-client-mu-s6>" >&2
  echo "A pasta precisa conter main.exe e os data folders do client Season 6." >&2
  exit 1
fi
if [[ ! -f "$CLIENT_DIR/main.exe" ]]; then
  echo "ERRO: $CLIENT_DIR não contém main.exe — não parece um client MU válido." >&2
  exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo ">> Copiando client de $CLIENT_DIR ..."
cp -a "$CLIENT_DIR/." "$WORK/"

echo ">> Baixando OpenMU ClientLauncher ..."
curl -fsSL "$LAUNCHER_URL" -o "$WORK/_launcher.zip"
mkdir -p "$WORK/_launcher"
unzip -o -q "$WORK/_launcher.zip" -d "$WORK/_launcher"
cp -a "$WORK/_launcher/." "$WORK/"
rm -rf "$WORK/_launcher" "$WORK/_launcher.zip"

# launcher.config placeholder na raiz — a engine SEMPRE sobrescreve este arquivo
# com o IP/porta do servidor antes de entregar o zip ao jogador.
cat > "$WORK/launcher.config" <<'XML'
<?xml version="1.0" encoding="UTF-8"?>
<LauncherSettings>
  <MainExePath>main.exe</MainExePath>
  <Hosts />
</LauncherSettings>
XML

echo ">> Compactando $OUT_ZIP ..."
rm -f "$OUT_ZIP"
( cd "$WORK" && zip -r -q "$OUT_ZIP" . )
echo ">> Pronto: $OUT_ZIP ($(du -h "$OUT_ZIP" | cut -f1))"
echo ">> A engine já serve este zip — teste o botão 'Baixar Client' no dashboard."
