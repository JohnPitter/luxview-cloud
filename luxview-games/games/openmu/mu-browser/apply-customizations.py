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


def append_once(path, marker, content):
    try:
        s = open(path, encoding="utf-8").read()
    except FileNotFoundError:
        print(f"MISSING {path}")
        return
    if marker in s:
        print(f"no-change {path} (append)")
        return
    open(path, "a", encoding="utf-8").write("\n" + content + "\n")
    print(f"appended {path}")


def patch(path, replacements, required=True):
    try:
        s = open(path, encoding="utf-8").read()
    except FileNotFoundError:
        if required:
            print(f"MISSING {path}")
        return
    orig = s
    for old, new in replacements:
        if new in s:
            continue  # already applied (idempotent)
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

# 3c) Punchier image (contrast + exposure) so colors pop closer to the native
#     client's warmer look. Applied post-process, safe to tune.
patch("src/scenes/testScene.ts", [
    ("this.ambientColor = new Color3(1, 1, 1);",
     "this.ambientColor = new Color3(1, 1, 1);\n    this.imageProcessingConfiguration.contrast = 1.35;\n    this.imageProcessingConfiguration.exposure = 1.2;"),
])

# 3d) Hide the in-world skill-test buttons (dev effect triggers) for a cleaner HUD.
patch("src/ui/pages/worldPage/index.tsx", [
    ("<Skills />", "{false && <Skills />}"),
])

# 3d-2) Position the browser as a preview: a clear CTA to download the full
#       Windows client (the complete experience — combat, classic HUD).
patch("src/ui/pages/preloaderPage/index.tsx", [
    ("""        <button onClick={() => Store.playOnline()}>Play Online</button>
      </div>""",
     """        <button onClick={() => Store.playOnline()}>Play Online</button>
      </div>
      <a
        href="https://mu.luxview.cloud/client/LuxViewMU-Windows.zip"
        style={{ display: 'block', marginTop: '18px', padding: '14px 24px', background: '#f5a623', color: '#1a1a1a', fontWeight: 'bold', textDecoration: 'none', borderRadius: '8px', textAlign: 'center', maxWidth: '380px' }}
      >
        🪟 Baixar Client Windows — experiência completa
      </a>"""),
])

# 3d-3) Polish the bottom HUD (life bars + buttons) toward a MU gold/bronze look
#       — the buttons were tiny 8px default browser buttons.
append_once(
    "src/ui/pages/worldPage/components/bottomBar/style.less",
    "LuxView HUD polish",
    """/* LuxView HUD polish */
.bottom-bar {
  .panel {
    background: linear-gradient(to bottom, #1a1612, #0b0907);
    border-top: 2px solid #b8860b;
    padding: 6px 0 4px;
  }
  .skills button,
  .buttons button {
    font-size: 12px;
    line-height: 1;
    color: #ffd9a0;
    background: linear-gradient(to bottom, #2c2218, #14100a);
    border: 1px solid #8a6d3b;
    border-radius: 4px;
    padding: 5px 3px;
    margin: 1px;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }
  .skills button:hover,
  .buttons button:hover {
    background: linear-gradient(to bottom, #3c2f20, #1d160d);
    border-color: #d4af37;
    color: #fff;
  }
  .vertical-bar .bg {
    border: 2px solid #6b4f1d;
    border-radius: 4px;
    box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.7);
  }
  .consumable-item {
    border-color: #6b4f1d !important;
  }
}""",
)

# 3e) Auto-play each model's idle animation on load (it was commented out) so
#     characters/monsters/NPCs animate instead of standing frozen — a big
#     "alive / like the native client" win. Verified via screenshots.
patch("src/common/modelLoader.ts", [
    ("""          // if (task.loadedAnimationGroups.length > 0) {
          //   task.loadedAnimationGroups[0].play(true);
          // }""",
     """          if (task.loadedAnimationGroups.length > 0) {
            task.loadedAnimationGroups[0].play(true);
          }"""),
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
