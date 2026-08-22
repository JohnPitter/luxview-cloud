#!/usr/bin/env python3
"""Account helper for native Priston 4220 (MSSQL {Letter}GameUser)."""
from __future__ import annotations

import json
import os
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SQLCMD = "/opt/mssql-tools18/bin/sqlcmd"


def mssql_host() -> str:
    return os.environ.get("PRISTON_MSSQL_HOST", "luxview-mssql")


def mssql_password() -> str:
    return os.environ.get("PRISTON_MSSQL_PASSWORD", "")


def quote(value: str) -> str:
    return "N'" + value.replace("'", "''") + "'"


def run_sql(sql: str) -> None:
    password = mssql_password()
    if not password:
        raise SystemExit("PRISTON_MSSQL_PASSWORD ausente")
    result = subprocess.run(
        [
            SQLCMD,
            "-S",
            mssql_host(),
            "-U",
            "sa",
            "-P",
            password,
            "-C",
            "-b",
            "-Q",
            sql,
        ],
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stdout + result.stderr)
        raise SystemExit(result.returncode)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def _send(self, code: int, body: dict) -> None:
        payload = json.dumps(body).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self) -> None:  # noqa: N802
        if self.path in ("/health", "/", "/healthz"):
            self._send(200, {"ok": True, "game": "priston-4220"})
            return
        if self.path.startswith("/players"):
            self._send(200, {"players": []})
            return
        self._send(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/register":
            self._send(404, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            data = json.loads(raw.decode("utf-8"))
        except json.JSONDecodeError:
            self._send(400, {"error": "json inválido"})
            return
        user = str(data.get("userid") or data.get("login") or "").strip()
        password = str(data.get("password") or "")
        if not user or not password:
            self._send(400, {"error": "userid/password ausentes"})
            return
        letter = user[0].upper()
        if letter < "A" or letter > "Z":
            letter = "P"
        table = letter + "GameUser"
        sql = (
            "USE [accountdb]; "
            f"IF EXISTS (SELECT 1 FROM [dbo].[{table}] WHERE [userid] = {quote(user)}) "
            f"BEGIN UPDATE [dbo].[{table}] SET [Passwd] = {quote(password)}, [inuse] = 0, "
            f"[BlockChk] = 0, [ServerName] = N'LuxView', [EditDay] = GETDATE() "
            f"WHERE [userid] = {quote(user)}; END ELSE BEGIN "
            f"INSERT INTO [dbo].[{table}] ([userid], [Passwd], [GameCode], [GPCode], "
            "[RegistDay], [DisuseDay], [UsePeriod], [Credit], [SelectChk], [EventChk], "
            "[BlockChk], [inuse], [DelChk], [ServerName], [EditDay], [RNo], [SNo], "
            f"[Channel], [BNum]) VALUES ({quote(user)}, {quote(password)}, N'0', N'0', GETDATE(), "
            "DATEADD(YEAR, 20, GETDATE()), 0, 0, 0, 0, 0, 0, 0, N'LuxView', GETDATE(), "
            "NULL, NULL, N'0', 0); END;"
        )
        try:
            run_sql(sql)
        except SystemExit as exc:
            self._send(500, {"error": "mssql recusou o insert", "code": int(exc.code or 1)})
            return
        self._send(200, {"ok": True, "userid": user})


def main() -> None:
    if len(sys.argv) > 1 and sys.argv[1] == "sql":
        sql = os.environ.get("PRISTON_SQL", "").strip()
        if not sql:
            raise SystemExit("PRISTON_SQL ausente")
        run_sql(sql)
        return
    if len(sys.argv) > 1 and sys.argv[1] == "serve":
        host = os.environ.get("PRISTON_ACCOUNT_BIND", "0.0.0.0")
        port = int(os.environ.get("PRISTON_ACCOUNT_PORT", "5080"))
        ThreadingHTTPServer((host, port), Handler).serve_forever()
        return
    raise SystemExit("uso: priston-account.py sql|serve")


if __name__ == "__main__":
    main()
