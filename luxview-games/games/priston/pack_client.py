#!/usr/bin/env python3
"""Pack native Brazil Priston Tale 4220 client for LuxView catalog."""
from __future__ import annotations

import os
import zipfile
from pathlib import Path

SRC = Path(
    r"C:\Users\joaop\Desenvolvimento\openpriston\client-runtime\PristonTale_Brazil_Client-v4220"
)
EXTRA_SRC = Path(
    r"C:\Users\joaop\Desenvolvimento\openpriston\dedicated server\client"
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
"ServerName" "LuxView Priston"
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


def collect_files(root: Path) -> list[Path]:
    files: list[Path] = []
    for current, dirs, names in os.walk(root):
        dirs[:] = [d for d in dirs if d.lower() not in SKIP_DIR_NAMES]
        for name in names:
            path = Path(current) / name
            rel = path.relative_to(root)
            if skip_file(rel):
                continue
            files.append(rel)
    return files


def patch_reg(path: Path) -> bytes:
    lines = path.read_text(encoding="ascii").splitlines()
    patched = []
    for line in lines:
        if line.startswith('"Server1"='):
            line = '"Server1"="187.77.227.65"'
        elif line.startswith('"Server2"='):
            line = '"Server2"="187.77.227.65"'
        elif line.startswith('"Server3"='):
            line = '"Server3"="187.77.227.65"'
        elif line.startswith('"Version"='):
            line = '"Version"="4220"'
        patched.append(line)
    return ("\n".join(patched) + "\n").encode("ascii")


def main() -> None:
    if not SRC.is_dir():
        raise SystemExit(f"client 4220 ausente: {SRC}")
    if not EXTRA_SRC.is_dir():
        raise SystemExit(f"client dedicado ausente: {EXTRA_SRC}")
    OUT.parent.mkdir(parents=True, exist_ok=True)
    if OUT.exists():
        OUT.unlink()

    files = collect_files(SRC)
    base_names = {rel.as_posix().lower() for rel in files}
    extra_files = [
        rel for rel in collect_files(EXTRA_SRC)
        if rel.as_posix().lower() not in base_names
    ]
    entries = [(SRC, rel) for rel in files] + [(EXTRA_SRC, rel) for rel in extra_files]
    entries.sort(key=lambda item: item[1].as_posix().lower())
    print(f"packing {len(entries)} files from {SRC} -> {OUT}", flush=True)
    if extra_files:
        print("dedicated-only extras: " + ", ".join(rel.as_posix() for rel in extra_files), flush=True)

    written = 0
    with zipfile.ZipFile(OUT, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=1) as zf:
        for source, rel in entries:
            arc = zip_name(rel)
            lower = rel.name.lower()
            if lower == "luncher.ini":
                zf.writestr(arc, LUNCHER_INI.encode("ascii"))
            elif lower == "ptreg.rgx":
                zf.writestr(arc, PTREG.encode("ascii"))
            elif lower == "jogar-servidor-local.reg":
                zf.writestr(arc, patch_reg(source / rel))
            else:
                zf.write(source / rel, arcname=arc)
            written += 1
            if written % 1000 == 0:
                print(f"  {written}/{len(entries)}", flush=True)

    size = OUT.stat().st_size
    print(f"done files={written} zip_bytes={size} zip_gb={size / 1024**3:.2f} path={OUT}", flush=True)


if __name__ == "__main__":
    main()
