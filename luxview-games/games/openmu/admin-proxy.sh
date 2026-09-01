#!/bin/bash
# Aguarda o Blazor admin em 127.0.0.1:5000 antes de abrir o proxy 18080→5000.
set -e

wait_for_5000() {
    local i
    for i in $(seq 1 180); do
        if bash -c 'echo >/dev/tcp/127.0.0.1/5000' 2>/dev/null; then
            echo "[admin-proxy] :5000 responde após ${i}s"
            return 0
        fi
        sleep 1
    done
    echo "[admin-proxy] :5000 ainda não responde após 180s; iniciando socat (retry no cliente)" >&2
    return 1
}

echo "[admin-proxy] aguardando OpenMU admin em 127.0.0.1:5000..."
wait_for_5000 || true
exec /usr/bin/socat TCP-LISTEN:18080,fork,reuseaddr,bind=0.0.0.0 TCP:127.0.0.1:5000
