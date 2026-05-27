#!/bin/bash
set -e

SERVER_DIR="/vrising-server"
DATA_DIR="/vrising-data"
STEAMCMD="/opt/steamcmd/steamcmd.sh"

export WINEPREFIX="$DATA_DIR/.wine"
export WINEDLLOVERRIDES="mscoree,mshtml="

mkdir -p "$SERVER_DIR" "$DATA_DIR/Settings" "$DATA_DIR/Saves"

# Load config overrides written by Luxview Games (takes precedence over env vars)
CONFIG_FILE="$DATA_DIR/server-config.env"
if [ -f "$CONFIG_FILE" ]; then
    set -a
    source "$CONFIG_FILE"
    set +a
fi

# Generate ServerGameSettings.json from VRGAME_* env vars.
# When custom settings are generated, -preset must NOT be passed to the server
# because -preset overrides ServerGameSettings.json entirely.
CUSTOM_SETTINGS=0
if env | grep -q '^VRGAME_'; then
    CUSTOM_SETTINGS=1
    python3 - <<PYEOF
import json, os

def flt(k, d): v = os.environ.get(k, ""); return float(v) if v else float(d)
def bol(k, d): v = os.environ.get(k, ""); return v.lower() in ('1','true','yes') if v else d
def itg(k, d): v = os.environ.get(k, ""); return int(v) if v else int(d)
def s(k, d):   return os.environ.get(k, "") or d

settings = {
    "GameDifficulty":                s("VRGAME_GAME_DIFFICULTY", "Normal"),
    "GameModeType":                  s("VRGAME_GAME_MODE_TYPE", "PvP"),
    "CastleDamageMode":              s("VRGAME_CASTLE_DAMAGE_MODE", "Never"),
    "PlayerDamageMode":              s("VRGAME_PLAYER_DAMAGE_MODE", "Always"),
    "CastleHeartDamageMode":         s("VRGAME_CASTLE_HEART_DAMAGE_MODE", "CanBeDestroyedByPlayers"),
    "PvPProtectionMode":             s("VRGAME_PVP_PROTECTION_MODE", "Medium"),
    "DeathContainerPermission":      s("VRGAME_DEATH_CONTAINER_PERMISSION", "Anyone"),
    "ClanSize":                      itg("VRGAME_CLAN_SIZE", 4),
    "AllowGlobalChat":               bol("VRGAME_ALLOW_GLOBAL_CHAT", True),
    "AllWaypointsUnlocked":          bol("VRGAME_ALL_WAYPOINTS_UNLOCKED", False),
    "BloodBoundEquipment":           bol("VRGAME_BLOOD_BOUND_EQUIPMENT", True),
    "TeleportBoundItems":            bol("VRGAME_TELEPORT_BOUND_ITEMS", True),
    "CanLootEnemyContainers":        bol("VRGAME_CAN_LOOT_ENEMY_CONTAINERS", True),
    "FreeCastleRaid":                bol("VRGAME_FREE_CASTLE_RAID", False),
    "FreeCastleClaim":               bol("VRGAME_FREE_CASTLE_CLAIM", False),
    "FreeCastleDestroy":             bol("VRGAME_FREE_CASTLE_DESTROY", False),
    "InactivityKillEnabled":         bol("VRGAME_INACTIVITY_KILL_ENABLED", True),
    "InventoryStacksModifier":       flt("VRGAME_INVENTORY_STACKS", 1.0),
    "DropTableModifier_General":     flt("VRGAME_DROP_RATE", 1.0),
    "MaterialYieldModifier_Global":  flt("VRGAME_MATERIAL_YIELD", 1.0),
    "BloodEssenceYieldModifier":     flt("VRGAME_BLOOD_ESSENCE_YIELD", 1.0),
    "CraftRateModifier":             flt("VRGAME_CRAFT_RATE", 1.0),
    "BuildCostModifier":             flt("VRGAME_BUILD_COST", 1.0),
    "RecipeCostModifier":            flt("VRGAME_RECIPE_COST", 1.0),
    "ResearchCostModifier":          flt("VRGAME_RESEARCH_COST", 1.0),
    "RefinementRateModifier":        flt("VRGAME_REFINEMENT_RATE", 1.0),
    "RefinementCostModifier":        flt("VRGAME_REFINEMENT_COST", 1.0),
    "DismantleResourceModifier":     flt("VRGAME_DISMANTLE_RESOURCE", 1.0),
    "ServantConvertRateModifier":    flt("VRGAME_SERVANT_CONVERT_RATE", 1.0),
    "RepairCostModifier":            flt("VRGAME_REPAIR_COST", 1.0),
    "BloodDrainModifier":            flt("VRGAME_BLOOD_DRAIN", 1.0),
    "DurabilityDrainModifier":       flt("VRGAME_DURABILITY_DRAIN", 1.0),
    "GarlicAreaStrengthModifier":    flt("VRGAME_GARLIC_STRENGTH", 1.0),
    "HolyAreaStrengthModifier":      flt("VRGAME_HOLY_STRENGTH", 1.0),
    "SilverStrengthModifier":        flt("VRGAME_SILVER_STRENGTH", 1.0),
    "SunDamageModifier":             flt("VRGAME_SUN_DAMAGE", 1.0),
    "CastleDecayRateModifier":       flt("VRGAME_CASTLE_DECAY_RATE", 1.0),
    "CastleBloodEssenceDrainModifier": flt("VRGAME_CASTLE_BLOOD_DRAIN", 1.0),
    "CastleRelocationEnabled":       bol("VRGAME_CASTLE_RELOCATION", True),
    "GameTimeModifiers": {
        "DayDurationInSeconds":      flt("VRGAME_DAY_DURATION", 1080.0),
        "DayStartHour":              itg("VRGAME_DAY_START_HOUR", 9),
        "DayStartMinute":            0,
        "DayEndHour":                itg("VRGAME_DAY_END_HOUR", 17),
        "DayEndMinute":              0,
        "BloodMoonFrequency_Min":    itg("VRGAME_BLOOD_MOON_MIN", 10),
        "BloodMoonFrequency_Max":    itg("VRGAME_BLOOD_MOON_MAX", 18),
        "BloodMoonBuff":             flt("VRGAME_BLOOD_MOON_BUFF", 0.2),
    },
    "VampireStatModifiers": {
        "MaxHealthModifier":         flt("VRGAME_VAMPIRE_HEALTH", 1.0),
        "PhysicalPowerModifier":     flt("VRGAME_VAMPIRE_PHYSICAL_POWER", 1.0),
        "SpellPowerModifier":        flt("VRGAME_VAMPIRE_SPELL_POWER", 1.0),
        "ResourcePowerModifier":     flt("VRGAME_VAMPIRE_RESOURCE_POWER", 1.0),
        "SiegePowerModifier":        flt("VRGAME_VAMPIRE_SIEGE_POWER", 1.0),
        "DamageReceivedModifier":    flt("VRGAME_VAMPIRE_DAMAGE_RECEIVED", 1.0),
    },
    "UnitStatModifiers_Global": {
        "MaxHealthModifier":         flt("VRGAME_UNIT_HEALTH", 1.0),
        "PowerModifier":             flt("VRGAME_UNIT_POWER", 1.0),
        "LevelIncrease":             itg("VRGAME_UNIT_LEVEL_INCREASE", 0),
    },
    "UnitStatModifiers_VBlood": {
        "MaxHealthModifier":         flt("VRGAME_VBLOOD_HEALTH", 1.0),
        "PowerModifier":             flt("VRGAME_VBLOOD_POWER", 1.0),
        "LevelIncrease":             itg("VRGAME_VBLOOD_LEVEL_INCREASE", 0),
    },
}

out = "$DATA_DIR/Settings/ServerGameSettings.json"
with open(out, "w") as f:
    json.dump(settings, f, indent=2)
print(f"[vrising] ServerGameSettings.json written to {out}")
PYEOF
fi

# Download or update V Rising Dedicated Server (App ID: 1829350).
# SteamCMD can return state 0x6 (installed but update failed) when Steam CDN is
# flaky; we tolerate that as long as the binary already exists in the volume.
echo "[vrising] Updating server files..."
"$STEAMCMD" \
    +@sSteamCmdForcePlatformType windows \
    +force_install_dir "$SERVER_DIR" \
    +login anonymous \
    +app_update 1829350 \
    +quit || true

if [ ! -f "$SERVER_DIR/VRisingServer.exe" ]; then
    echo "[vrising] ERROR: VRisingServer.exe not found after update attempt, aborting"
    exit 1
fi

# Virtual display required by Wine/Unity
Xvfb :1 -screen 0 1024x768x16 &
export DISPLAY=:1.0
sleep 3

# Init Wine prefix only on first run
if [ ! -f "$WINEPREFIX/system.reg" ]; then
    echo "[vrising] Initializing Wine prefix (first run)..."
    wineboot --init || true
    wineserver -w
fi

echo "[vrising] Starting V Rising Dedicated Server..."
# When custom VRGAME_* settings are active, omit -preset so ServerGameSettings.json is used.
# The -preset flag overrides ServerGameSettings.json entirely, so it must not be passed together.
if [ "$CUSTOM_SETTINGS" = "0" ]; then
    PRESET_FLAG="${VRISING_PRESET:+-preset $VRISING_PRESET}"
else
    PRESET_FLAG=""
fi

exec wine "$SERVER_DIR/VRisingServer.exe" \
    -persistentDataPath "$DATA_DIR" \
    -serverName "${VRISING_SERVER_NAME:-V Rising Server}" \
    ${VRISING_DESCRIPTION:+-description "$VRISING_DESCRIPTION"} \
    -saveName "${VRISING_SAVE_NAME:-world1}" \
    -gamePort "${VRISING_GAME_PORT:-27015}" \
    -queryPort "${VRISING_QUERY_PORT:-27016}" \
    ${VRISING_PASSWORD:+-password "$VRISING_PASSWORD"} \
    ${PRESET_FLAG} \
    ${VRISING_DIFFICULTY_PRESET:+-difficultyPreset "$VRISING_DIFFICULTY_PRESET"} \
    -maxUsers "${VRISING_MAX_USERS:-40}" \
    -maxAdmins "${VRISING_MAX_ADMINS:-4}" \
    -fps "${VRISING_FPS:-30}" \
    -lowerFPSWhenEmpty "${VRISING_LOWER_FPS_WHEN_EMPTY:-true}" \
    -lowerFPSWhenEmptyValue "${VRISING_LOWER_FPS_WHEN_EMPTY_VALUE:-15}" \
    -secure "${VRISING_SECURE:-true}" \
    -listOnSteam "${VRISING_LIST_ON_STEAM:-true}" \
    -listOnEOS "${VRISING_LIST_ON_EOS:-true}" \
    -saveCount "${VRISING_SAVE_COUNT:-20}" \
    -saveInterval "${VRISING_SAVE_INTERVAL:-120}" \
    -resetDaysInterval "${VRISING_RESET_DAYS_INTERVAL:-0}" \
    -dayOfReset "${VRISING_DAY_OF_RESET:-Any}" \
    ${VRISING_RCON_ENABLED:+-rconEnabled "$VRISING_RCON_ENABLED"} \
    ${VRISING_RCON_PORT:+-rconPort "$VRISING_RCON_PORT"} \
    ${VRISING_RCON_PASSWORD:+-rconPassword "$VRISING_RCON_PASSWORD"}
