# syntax=docker/dockerfile:1
# LuxView Priston Tale 4220: runtime Wine32 comprovado pelo legacy-docker.
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    WINEARCH=win32 \
    WINEPREFIX=/wine \
    WINEDEBUG=-all \
    WINEDLLOVERRIDES="mscoree,mshtml=;msvcr70,mfc70=n;msado15,oledb32,msdasql,msdaps,msdaenum,msdatl3=n;odbc32,odbccp32,odbcint=b;d3dx9_43,d3dx9_35=n;d3d9=b" \
    PRISTON_SERVER_ROOT=/server

RUN dpkg --add-architecture i386 && apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates cabextract curl gnupg iproute2 net-tools procps python3 socat wget wine wine32 winbind \
    xauth xvfb xz-utils libgl1:i386 libgl1-mesa-dri:i386 libglu1-mesa:i386 libosmesa6:i386 unixodbc:i386 \
    && curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor -o /usr/share/keyrings/microsoft-prod.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/microsoft-prod.gpg] https://packages.microsoft.com/debian/12/prod bookworm main" > /etc/apt/sources.list.d/mssql-release.list \
    && apt-get update && ACCEPT_EULA=Y apt-get install -y --no-install-recommends mssql-tools18 unixodbc-dev \
    && rm -rf /var/lib/apt/lists/*

COPY vendor/MDAC_TYP.EXE /opt/mdac/MDAC_TYP.EXE
COPY vendor/*.cab /opt/vendor/
COPY install-mdac-manual.sh bake-prefix.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/install-mdac-manual.sh /usr/local/bin/bake-prefix.sh \
    && mkdir -p /wine /server /data/state /artifacts \
    && /usr/local/bin/bake-prefix.sh

COPY entrypoint.sh /entrypoint.sh
COPY priston-account.py /opt/priston-account.py
COPY patch_sql_dll.py /opt/patch_sql_dll.py
COPY init-accountdb.sql /opt/init-accountdb.sql
RUN sed -i 's/\r$//' /entrypoint.sh /opt/priston-account.py /opt/patch_sql_dll.py \
    && chmod +x /entrypoint.sh /opt/priston-account.py /opt/patch_sql_dll.py

EXPOSE 10012/tcp 10013/tcp 5080/tcp
ENTRYPOINT ["/entrypoint.sh"]
