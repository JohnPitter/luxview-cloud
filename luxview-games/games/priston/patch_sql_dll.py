#!/usr/bin/env python3
"""Retarget sql.dll SQLOLEDB Data Source without changing the PE layout."""
from __future__ import annotations

import argparse
from pathlib import Path

ORIGINAL = b"Provider=SQLOLEDB;Initial Catalog=accountdb;Data Source=SUNNY-PT\\SQLEXPRESS"

LOCAL_CHANNEL = b"SELECT 1 PRISTONTALE_ID,'GamerLocal' TABLE_NAME"
LOCAL_BILLING = (
    b"SELECT 1 NUM,'LOCAL' EXTERNAL_ID,'0' GOODS_TYPE,GETDATE() START_DAY,"
    b"DATEADD(year,10,GETDATE()) END_DAY,0 LEFT_TIME,'0' CHARGE_FORM"
)
SELECT_ONE = b"SELECT 1"


def put_ascii(data: bytearray, offset: int, capacity: int, value: bytes) -> None:
    if len(value) > capacity:
        raise SystemExit(f"valor de {len(value)} bytes nao cabe em {capacity} @ {offset:#x}")
    for i in range(capacity):
        data[offset + i] = 0
    data[offset : offset + len(value)] = value


def patch(src: Path, dst: Path, server: str) -> None:
    replacement = f"Provider=SQLOLEDB;Initial Catalog=accountdb;Data Source={server}".encode("ascii")
    if len(replacement) > len(ORIGINAL):
        raise SystemExit("Data Source novo nao cabe no slot original da sql.dll")

    data = bytearray(src.read_bytes())
    idx = data.find(ORIGINAL)
    if idx < 0:
        # Already retargeted from original; find any SQLOLEDB accountdb slot.
        prefix = b"Provider=SQLOLEDB;Initial Catalog=accountdb;Data Source="
        idx = data.find(prefix)
        if idx < 0:
            raise SystemExit("sql.dll sem connection string SQLOLEDB/accountdb")
        end = data.find(b"\x00", idx)
        if end < 0 or end - idx > len(ORIGINAL) + 8:
            end = idx + len(ORIGINAL)
        slot = end - idx
        if len(replacement) > slot:
            raise SystemExit("connection string nova maior que o slot atual")
        for i in range(slot):
            data[idx + i] = 0
        data[idx : idx + len(replacement)] = replacement
    else:
        for i in range(len(ORIGINAL)):
            data[idx + i] = 0
        data[idx : idx + len(replacement)] = replacement

    # Local billing stubs (same offsets as database/patch_sql_dll.ps1).
    try:
        put_ascii(data, 0x502F0, 100, LOCAL_CHANNEL)
        put_ascii(data, 0x525B8, 231, LOCAL_BILLING)
        put_ascii(data, 0x526D0, 165, LOCAL_BILLING)
        put_ascii(data, 0x52810, 96, SELECT_ONE)
        put_ascii(data, 0x52888, 97, SELECT_ONE)
        put_ascii(data, 0x52918, 115, SELECT_ONE)
        put_ascii(data, 0x529A8, 116, SELECT_ONE)
    except (IndexError, SystemExit) as exc:
        print(f"aviso billing stub: {exc}")

    dst.write_bytes(data)
    print(f"sql.dll -> {dst} Data Source={server}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--src", required=True)
    parser.add_argument("--dst", required=True)
    parser.add_argument("--server", required=True)
    args = parser.parse_args()
    patch(Path(args.src), Path(args.dst), args.server)


if __name__ == "__main__":
    main()
