#!/bin/bash
# Aplica taxas do dashboard no Postgres do OpenMU e só então sobe o processo.
# Tabelas OpenMU vivem em config/data (quoted); search_path é public.
set -e

num() {
    local name="$1" value="$2" fallback="$3"
    if [[ "$value" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        printf '%s' "$value"
    else
        printf '%s' "$fallback"
    fi
}

OPENMU_EXP_RATE="$(num EXP "${OPENMU_EXP_RATE:-10}" 10)"
OPENMU_MASTER_EXP_RATE="$(num MASTER "${OPENMU_MASTER_EXP_RATE:-10}" 10)"
OPENMU_ZEN_RATE="$(num ZEN "${OPENMU_ZEN_RATE:-5}" 5)"
OPENMU_EXCELLENT_DELTA="$(num EXC "${OPENMU_EXCELLENT_DELTA:-0}" 0)"
OPENMU_MAX_LEVEL="$(num MAX "${OPENMU_MAX_LEVEL:-400}" 400)"
OPENMU_MAX_MASTER_LEVEL="$(num MMAX "${OPENMU_MAX_MASTER_LEVEL:-400}" 400)"
OPENMU_MAX_PARTY="$(num PARTY "${OPENMU_MAX_PARTY:-5}" 5)"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-openmu}"
PGHOST="${PGHOST:-${DB_HOST:-localhost}}"

export PGPASSWORD="$POSTGRES_PASSWORD"

wait_pg() {
    local i
    for i in $(seq 1 60); do
        if psql -h "$PGHOST" -U postgres -d openmu -c 'SELECT 1' >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done
    echo "[openmu] postgres não respondeu; subindo sem aplicar taxas" >&2
    return 1
}

try_sql() {
    psql -h "$PGHOST" -U postgres -d openmu -v ON_ERROR_STOP=1 -c "$1" >/dev/null 2>&1
}

# to_regclass does not raise "relation does not exist" — unlike SELECT FROM.
schema_ready() {
    local r
    r="$(psql -h "$PGHOST" -U postgres -d openmu -tAc "SELECT to_regclass('config.\"GameConfiguration\"')" 2>/dev/null || true)"
    [[ "$r" == *GameConfiguration* ]]
}

apply_server_column() {
    local sid="$1" column="$2" varname="$3"
    local value
    value="$(eval "printf '%s' \"\${$varname:-}\"")"
    if [[ "$value" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        try_sql "UPDATE config.\"GameServerDefinition\" SET \"$column\" = ${value} WHERE \"ServerID\" = ${sid};" || true
    fi
}

# Item/Zen/Jewel percents are also editable in the OpenMU admin Drops page.
# Seed only when still unset/legacy multiplier (≤ 1) so dashboard and admin don't fight.
apply_server_drop_if_unset() {
    local sid="$1" column="$2" varname="$3"
    local value
    value="$(eval "printf '%s' \"\${$varname:-}\"")"
    if [[ "$value" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
        try_sql "UPDATE config.\"GameServerDefinition\" SET \"$column\" = ${value} WHERE \"ServerID\" = ${sid} AND (\"$column\" IS NULL OR \"$column\" <= 1);" || true
    fi
}

cleanup_dummy_plugins() {
    try_sql "DELETE FROM config.\"PlugInConfiguration\" WHERE \"TypeId\" IN ('a1b2c3d4-e5f6-4789-a012-3456789abcde', '434fa305-edc2-38bf-8dcb-65657e279e26', 'b17e8c4a-2d91-4f06-9a33-6c0e1b7d4a21');" || true
}

apply_server_rates() {
    local sid prefix
    for sid in 0 20 40; do
        prefix="OPENMU_S${sid}_"
        apply_server_drop_if_unset "$sid" "ItemDropRate" "${prefix}ITEM_DROP"
        apply_server_drop_if_unset "$sid" "ZenDropRate" "${prefix}ZEN_DROP"
        apply_server_drop_if_unset "$sid" "JewelDropRate" "${prefix}JEWEL_DROP"
        apply_server_column "$sid" "MixSuccessRateMultiplier" "${prefix}MIX_MULT"
        apply_server_column "$sid" "BlessUpgradeSuccessRate" "${prefix}BLESS_RATE"
        apply_server_column "$sid" "SoulUpgradeSuccessRate" "${prefix}SOUL_RATE"
        apply_server_column "$sid" "SoulUpgradeLuckBonusRate" "${prefix}SOUL_LUCK"
        apply_server_column "$sid" "LifeUpgradeSuccessRate" "${prefix}LIFE_RATE"
        apply_server_column "$sid" "HarmonyUpgradeSuccessRate" "${prefix}HARMONY_RATE"
        apply_server_column "$sid" "LowerRefineSuccessRate" "${prefix}REFINE_LOW"
        apply_server_column "$sid" "HigherRefineSuccessRate" "${prefix}REFINE_HIGH"
        apply_server_column "$sid" "MixPlus10SuccessRate" "${prefix}MIX_PLUS10"
        apply_server_column "$sid" "MixPlus11SuccessRate" "${prefix}MIX_PLUS11"
    done
}

apply_rates() {
    # Seed only for global GameConfiguration. Admin (or a previous seed) wins — never reset on every restart.
    try_sql "UPDATE config.\"GameConfiguration\" SET \"ExperienceRate\" = ${OPENMU_EXP_RATE} WHERE \"ExperienceRate\" IS NULL OR \"ExperienceRate\" <= 1" || return 1
    try_sql "UPDATE config.\"GameConfiguration\" SET \"MasterExperienceRate\" = ${OPENMU_MASTER_EXP_RATE} WHERE \"MasterExperienceRate\" IS NULL OR \"MasterExperienceRate\" <= 1" || true
    try_sql "UPDATE config.\"GameConfiguration\" SET \"MaximumLevel\" = ${OPENMU_MAX_LEVEL} WHERE \"MaximumLevel\" IS NULL OR \"MaximumLevel\" <= 0" || true
    # Only seed S6: update 149 sets MaximumMasterLevel=0 on 99d/S2 (no ML on those channels).
    try_sql "UPDATE config.\"GameConfiguration\" SET \"MaximumMasterLevel\" = ${OPENMU_MAX_MASTER_LEVEL} WHERE \"Id\" = '00000001-0001-0000-0000-000000000000' AND (\"MaximumMasterLevel\" IS NULL OR \"MaximumMasterLevel\" <= 0)" || true
    try_sql "UPDATE config.\"GameConfiguration\" SET \"MaximumPartySize\" = ${OPENMU_MAX_PARTY} WHERE \"MaximumPartySize\" IS NULL OR \"MaximumPartySize\" <= 0" || true
    try_sql "UPDATE config.\"GameConfiguration\" SET \"ExcellentItemDropLevelDelta\" = ${OPENMU_EXCELLENT_DELTA} WHERE \"ExcellentItemDropLevelDelta\" IS NULL" || true
    # MoneyAmountRate is config.ConstValueAttribute.DefinitionId (GUID Stats.MoneyAmountRate), not data.
    try_sql "UPDATE config.\"ConstValueAttribute\" SET \"Value\" = ${OPENMU_ZEN_RATE} WHERE \"DefinitionId\" = 'd84d1a5c-3a56-4cb9-8dd4-158afd4d1edb' AND \"Value\" <= 1 AND \"GameConfigurationId\" IS NOT NULL" || true
    apply_server_rates
    return 0
}

echo "[openmu] aguardando postgres para aplicar taxas (exp ${OPENMU_EXP_RATE}x zen ${OPENMU_ZEN_RATE}x)..."
if ! wait_pg; then
    :
elif ! schema_ready; then
    echo "[openmu] schema ainda não existe — OpenMU sobe com defaults e as taxas valem no próximo restart"
elif cleanup_dummy_plugins && apply_rates; then
    echo "[openmu] config.GameConfiguration + GameServerDefinition atualizados (plugins dummy removidos)"
else
    echo "[openmu] schema existe mas o seed falhou — OpenMU sobe; conferir logs do postgres" >&2
fi

cd /opt/openmu
export ASPNETCORE_URLS="${ASPNETCORE_URLS:-http://127.0.0.1:5000}"
ARGS=(-autostart)
if [ -n "${OPENMU_RESOLVE_IP:-}" ]; then
    ARGS+=("-resolveIP:${OPENMU_RESOLVE_IP}")
fi
exec dotnet MUnique.OpenMU.Startup.dll "${ARGS[@]}"
