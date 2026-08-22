#!/bin/sh
# LuxView Priston Tale — aplica IP/nome/rates e sobe o OpenPriston.Server.
set -eu

mkdir -p /artifacts /data/state/character-data /client /app/captures

public_ip="${LUXVIEW_PUBLIC_IP:-${PRISTON_SERVER_IP:-127.0.0.1}}"
server_name="${PRISTON_SERVER_NAME:-LuxView}"
game_port="${LUXVIEW_GAME_PORT:-${PRISTON_ADVERTISED_PORT:-10012}}"
clan_port="${PRISTON_CLAN_PORT:-10013}"
rate_exp="${PRISTON_RATE_EXP:-5}"
rate_gold="${PRISTON_RATE_GOLD:-3}"
rate_drop="${PRISTON_RATE_DROP:-2}"
max_mobs="${PRISTON_MAX_MOBS:-32}"
spawn_batch="${PRISTON_SPAWN_BATCH:-4}"
spawn_protect="${PRISTON_SPAWN_PROTECTION:-10}"
client_root="${PRISTON_CLIENT_ROOT:-/client}"

export ASPNETCORE_URLS="${ASPNETCORE_URLS:-http://0.0.0.0:5080}"
export Gateway__ListenAddress=0.0.0.0
export Gateway__ListenPort=10012
export Gateway__AdvertisedAddress="$public_ip"
export Gateway__AdvertisedPort="$game_port"
export Gateway__ServerName="$server_name"
export Gateway__ClanPort="$clan_port"
export Gateway__CaptureDirectory=/app/captures
export CharacterData__RootDirectory=/data/state/character-data
export CharacterState__Path=/data/state/character-positions.json
export CharacterInventory__Path=/data/state/character-inventory.json
export CharacterProgress__Path=/data/state/character-progress.json
export QuestProgress__Path=/data/state/character-quests.json
export WorldSnapshot__ClientRootDirectory="$client_root"
export GameRates__Experience="$rate_exp"
export GameRates__Gold="$rate_gold"
export GameRates__Drop="$rate_drop"
export WorldSnapshot__MaxVisibleEntities="$max_mobs"
export WorldSnapshot__SpawnBatchSize="$spawn_batch"
export MobCombat__SpawnProtectionSeconds="$spawn_protect"

if [ ! -f "$client_root/Field/forest/fore-2.smd" ]; then
  echo "[priston] client ausente em $client_root (precisa Field/ e Char/ do client 4420/5421)" >&2
fi

echo "[priston] LuxView $server_name em ${public_ip}:${game_port} (exp ${rate_exp}x gold ${rate_gold}x drop ${rate_drop}x spots ${max_mobs})"
exec dotnet /app/OpenPriston.Server.dll
