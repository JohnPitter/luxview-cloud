#!/bin/bash
# Aplica taxas do dashboard no Postgres do OpenMU e só então sobe o processo.
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

apply_rates() {
    try_sql "UPDATE \"GameConfiguration\" SET \"ExperienceRate\" = ${OPENMU_EXP_RATE}" || return 1
    try_sql "UPDATE \"GameConfiguration\" SET \"MasterExperienceRate\" = ${OPENMU_MASTER_EXP_RATE}" || true
    try_sql "UPDATE \"GameConfiguration\" SET \"MaximumLevel\" = ${OPENMU_MAX_LEVEL}" || true
    try_sql "UPDATE \"GameConfiguration\" SET \"MaximumMasterLevel\" = ${OPENMU_MAX_MASTER_LEVEL}" || true
    try_sql "UPDATE \"GameConfiguration\" SET \"MaximumPartySize\" = ${OPENMU_MAX_PARTY}" || true
    try_sql "UPDATE \"GameConfiguration\" SET \"ExcellentItemDropLevelDelta\" = ${OPENMU_EXCELLENT_DELTA}" || true
    try_sql "UPDATE \"ConstValueAttribute\" AS c SET \"Value\" = ${OPENMU_ZEN_RATE} FROM \"AttributeDefinition\" AS a WHERE c.\"AttributeDefinitionId\" = a.\"Id\" AND (a.\"Designation\" ILIKE '%MoneyAmount%' OR a.\"Designation\" ILIKE '%Money%Rate%')" || true
    return 0
}

echo "[openmu] aguardando postgres para aplicar taxas (exp ${OPENMU_EXP_RATE}x zen ${OPENMU_ZEN_RATE}x)..."
if wait_pg && apply_rates; then
    echo "[openmu] GameConfiguration atualizado"
else
    echo "[openmu] schema ainda não existe — OpenMU sobe com defaults e as taxas valem no próximo restart"
fi

cd /opt/openmu
exec dotnet MUnique.OpenMU.Startup.dll -autostart
