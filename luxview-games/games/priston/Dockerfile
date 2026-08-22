# syntax=docker/dockerfile:1
# =============================================================================
# LuxView Priston Tale 4220 — native SunnyBPT_v4220.exe under Wine32 + xvfb
#
# Build context is this directory:
#   docker build -t luxview-cloud-priston:latest \
#     -f luxview-games/games/priston/Dockerfile \
#     luxview-games/games/priston
#
# GameServer/Field/char stay on the host volume:
#   /data/luxview/storage/_global/priston-assets/server-4220 -> /server
# MSSQL is a sidecar on luxview-net (luxview-mssql), not baked here.
# =============================================================================
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    WINEARCH=win32 \
    WINEPREFIX=/wine \
    WINEDEBUG=-all \
    WINEDLLOVERRIDES=mscoree,mshtml= \
    PRISTON_SERVER_ROOT=/server

RUN dpkg --add-architecture i386 \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        cabextract \
        curl \
        gnupg \
        iproute2 \
        python3 \
        wget \
        wine32 \
        winbind \
        xvfb \
        xz-utils \
    && curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor -o /usr/share/keyrings/microsoft-prod.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/microsoft-prod.gpg] https://packages.microsoft.com/debian/12/prod bookworm main" > /etc/apt/sources.list.d/mssql-release.list \
    && apt-get update \
    && ACCEPT_EULA=Y apt-get install -y --no-install-recommends mssql-tools18 unixodbc-dev \
    && rm -rf /var/lib/apt/lists/*

RUN wget -qO /usr/local/bin/winetricks https://raw.githubusercontent.com/Winetricks/winetricks/master/src/winetricks \
    && chmod +x /usr/local/bin/winetricks \
    && xvfb-run -a wineboot --init \
    && xvfb-run -a winetricks -q --force mdac28 \
    && wineserver -w

COPY entrypoint.sh /entrypoint.sh
COPY priston-account.py /opt/priston-account.py
COPY patch_sql_dll.py /opt/patch_sql_dll.py
COPY init-accountdb.sql /opt/init-accountdb.sql
RUN sed -i 's/\r$//' /entrypoint.sh /opt/priston-account.py /opt/patch_sql_dll.py \
    && chmod +x /entrypoint.sh /opt/priston-account.py /opt/patch_sql_dll.py \
    && mkdir -p /data/state /server /artifacts

EXPOSE 10012/tcp 10013/tcp 5080/tcp
ENTRYPOINT ["/entrypoint.sh"]
