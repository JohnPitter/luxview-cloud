#!/usr/bin/env python3
"""Apply LuxView customizations to a fresh muonlinejs clone (idempotent).

Run from the muonlinejs root after `git clone`/`git pull`:
    python3 apply-customizations.py

Covers: server/proxy config, render-quality tweaks (closer to the native
client) and the proxy allowlist (locked to our OpenMU).
"""
import re
import sys

CS_HOST = "187.77.227.65"
WS_HOST = "wss://muproxy.luxview.cloud"
WS_PORT = "443"


def patch(path, replacements, required=True):
    try:
        s = open(path, encoding="utf-8").read()
    except FileNotFoundError:
        if required:
            print(f"MISSING {path}")
        return
    orig = s
    for old, new in replacements:
        if old in s:
            s = s.replace(old, new)
    if s != orig:
        open(path, "w", encoding="utf-8").write(s)
        print(f"patched {path}")
    else:
        print(f"no-change {path}")


# 1) Point the client at our server + proxy.
patch("src/consts.ts", [
    ("export const CS_HOST = '127.0.0.1';", f"export const CS_HOST = '{CS_HOST}';"),
    ("export const WS_HOST = 'ws://localhost';", f"export const WS_HOST = '{WS_HOST}';"),
    ("export const WS_PORT = 3000;", f"export const WS_PORT = {WS_PORT};"),
])

# 2) Render quality: force AA, prefer the discrete GPU, render at native res.
patch("src/libs/babylon/utils.ts", [
    ("    !!enableAntialiasing,", "    true, // LuxView: always antialias"),
    ("powerPreference: 'low-power',", "powerPreference: 'high-performance',"),
    ("limitDeviceRatio: enableAntialiasing ? undefined : 1,", "limitDeviceRatio: undefined,"),
])

# 3) Sharper textures at oblique angles (terrain/objects): aniso 1 -> 16.
for f in ("src/common/BMD/createMeshes.ts", "src/common/utils.ts"):
    patch(f, [("anisotropicFilteringLevel = 1;", "anisotropicFilteringLevel = 16;")])

# 3b) Hide the dev debug overlay (coords/tile/flags + Save Pos/Exit buttons)
#     so it looks like a real client instead of a demo.
patch("src/App.tsx", [
    ("{state === UIState.World && <Debug />}",
     "{false && state === UIState.World && <Debug />}"),
])

# 4) Lock the WS->TCP proxy to our OpenMU (no open relay).
proxy = "proxy/main.ts"
try:
    s = open(proxy, encoding="utf-8").read()
    anchor = 'const targetPort = parseInt(searchParams.get("port") ?? "0");'
    if "ALLOWED_HOST" in s:
        print(f"no-change {proxy} (allowlist present)")
    elif anchor in s:
        check = anchor + '''

    const ALLOWED_HOST = "''' + CS_HOST + '''";
    const ALLOWED_PORTS = new Set([44405, 55901, 55902, 55903, 55904, 55905, 55906, 55980]);
    if (targetHost !== ALLOWED_HOST || !ALLOWED_PORTS.has(targetPort)) {
      return new Response("Forbidden target", { status: 403 });
    }'''
        open(proxy, "w", encoding="utf-8").write(s.replace(anchor, check, 1))
        print(f"patched {proxy}")
    else:
        print(f"ANCHOR-NOT-FOUND {proxy}")
except FileNotFoundError:
    print(f"MISSING {proxy}")

print("done")
