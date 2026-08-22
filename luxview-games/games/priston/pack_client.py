#!/usr/bin/env python3
"""Pack native Brazil Priston Tale 4220 client (SunnyBPT) for LuxView catalog."""
from __future__ import annotations

import os
import zipfile
from pathlib import Path

SRC = Path(
    r"C:\Users\joaop\Desenvolvimento\openpriston\client-runtime\PristonTale_Brazil_Client-v4220"
)
OUT = Path(
    r"C:\Users\joaop\Desenvolvimento\Projects\luxview-cloud\output\priston-4220-base.zip"
)

SKIP_DIR_NAMES = {"savedata"}
SKIP_SUFFIXES = {".original", ".bak", ".dmp", ".tmp", ".log"}
SKIP_FILE_NAMES = {
    "client-current.png",
    "sunnybpt-handshake-test.exe",
    "sunnybpt-nongm.exe",
    "sunnybpt.before-ui-patch-restore.exe",
    "sunnybpt.exe.pre-menu-click-patch",
    "sunnybpt.exe.pre-registration-patch",
}
SKIP_CONTAINS = (
    "handshake-test",
    "before-ui-patch",
    ".pre-menu-click",
    ".pre-registration",
    "-nongm",
)

LUNCHER_INI = """[INITGAME]
gameServerPORT=10012
gameServerIP=187.77.227.65
"""

PTREG = '''"Version" "4220"
"Graphic" "1"
"Network" "0"
"ScreenSize" "3"
"ColorBPP" "32"
"MotionBlur" "true"
"CameraSight" "ON"
"Sound" "Off"
"CameraInvert" "false"
"FullMode" "Off"
"MicOption" "OFF"
"Server1" "187.77.227.65"
"Server2" "187.77.227.65"
"Server3" "187.77.227.65"
"ServerName" "LuxView"
"Account" ""
"" ""
'''


def skip_file(rel: Path) -> bool:
    name = rel.name
    lower = name.lower()
    if lower in SKIP_FILE_NAMES:
        return True
    if rel.suffix.lower() in SKIP_SUFFIXES:
        return True
    if any(token in lower for token in SKIP_CONTAINS):
        return True
    if any(part.lower() in SKIP_DIR_NAMES for part in rel.parts[:-1]):
        return True
    return False


def zip_name(rel: Path) -> str:
    return rel.as_posix()


def main() -> None:
    if not SRC.is_dir():
        raise SystemExit(f"client 4220 ausente: {SRC}")
    OUT.parent.mkdir(parents=True, exist_ok=True)
    if OUT.exists():
        OUT.unlink()

    files: list[Path] = []
    for root, dirs, names in os.walk(SRC):
        dirs[:] = [d for d in dirs if d.lower() not in SKIP_DIR_NAMES]
        for name in names:
            path = Path(root) / name
            rel = path.relative_to(SRC)
            if skip_file(rel):
                continue
            files.append(rel)

    files.sort(key=lambda p: p.as_posix().lower())
    print(f"packing {len(files)} files from {SRC} -> {OUT}", flush=True)

    written = 0
    with zipfile.ZipFile(OUT, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=1) as zf:
        for rel in files:
            arc = zip_name(rel)
            lower = rel.name.lower()
            if lower == "luncher.ini":
                zf.writestr(arc, LUNCHER_INI.encode("ascii"))
            elif lower == "ptreg.rgx":
                zf.writestr(arc, PTREG.encode("ascii"))
            else:
                zf.write(SRC / rel, arcname=arc)
            written += 1
            if written % 1000 == 0:
                print(f"  {written}/{len(files)}", flush=True)

    size = OUT.stat().st_size
    print(f"done files={written} zip_bytes={size} zip_gb={size / 1024**3:.2f} path={OUT}", flush=True)


if __name__ == "__main__":
    main()
