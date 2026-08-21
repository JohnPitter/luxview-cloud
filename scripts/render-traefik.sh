#!/usr/bin/env bash
# Renders traefik/traefik.runtime.yml with INTERNAL_TOKEN from the environment or .env.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if [ -z "${INTERNAL_TOKEN:-}" ] && [ -f .env ]; then
  INTERNAL_TOKEN="$(grep '^INTERNAL_TOKEN=' .env | cut -d= -f2- | tr -d '\r')"
  export INTERNAL_TOKEN
fi
if [ -z "${INTERNAL_TOKEN:-}" ]; then
  echo "INTERNAL_TOKEN is required to render Traefik config" >&2
  exit 1
fi
sed "s/__INTERNAL_TOKEN__/${INTERNAL_TOKEN}/g" traefik/traefik.yml > traefik/traefik.runtime.yml
echo "wrote traefik/traefik.runtime.yml"
