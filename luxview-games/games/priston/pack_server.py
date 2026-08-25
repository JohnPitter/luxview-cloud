#!/usr/bin/env python3
"""Pack native Brazil Priston Tale 4220 server runtime for the VPS volume."""
from __future__ import annotations

import os
import tarfile
from pathlib import Path

SRC = Path(
    r"C:\Users\joaop\Desenvolvimento\openpriston\server-runtime\PristonTale_Brazil_Server-v4220"
)
OUT = Path(
    r"C:\Users\joaop\Desenvolvimento\Projects\luxview-cloud\output\priston-4220-server.tar"
)

SKIP_DIR_NAMES = {"_disabled-content", "pk_log"}
SKIP_SUFFIXES = {".dmp", ".log", ".bak", ".tmp"}
SKIP_FILE_NAMES = {
    "sunnybpt_port10014.exe",
    "sunnybpt_port10014.ini-default.exe",
    "sunnybpt_proxy.exe",
    "sql.dll.corrupted-variable-length-patch",
    "sql.dll.fixed-test",
    "sql.dll.pre-docker-host-patch",
    "pristonsqldll.dll.before-local-adapter",
    "legacy-server.log",
    "legacy-server2.log",
    "sunny-smoke.log",
}
# The source tree's adapted PristonSQLDll.dll is required. The official DLL
# fails silently in SQLLoginProcess/SQLLogoutProcess, so never substitute it.
SKIP_CONTAINS = ("crash", ".before-", "corrupted")


def skip_file(rel: Path) -> bool:
    lower = rel.name.lower()
    if lower in SKIP_FILE_NAMES:
        return True
    if rel.suffix.lower() in SKIP_SUFFIXES:
        return True
    if any(token in lower for token in SKIP_CONTAINS):
        return True
    if any(part.lower() in SKIP_DIR_NAMES for part in rel.parts[:-1]):
        return True
    return False


def main() -> None:
    if not SRC.is_dir():
        raise SystemExit(f"server 4220 ausente: {SRC}")
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

    with tarfile.open(OUT, "w") as tar:
        for i, rel in enumerate(files, 1):
            src_path = SRC / rel
            tar.add(src_path, arcname=rel.as_posix())
            if i % 1000 == 0:
                print(f"  {i}/{len(files)}", flush=True)

    size = OUT.stat().st_size
    print(f"done files={len(files)} tar_bytes={size} tar_gb={size / 1024**3:.2f}", flush=True)


if __name__ == "__main__":
    main()
