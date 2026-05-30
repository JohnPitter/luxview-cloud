# Rakion (SoftNyx v258) — servidor privado para LuxView Cloud.
#
# Imagem self-contained: broker + world (sob Wine win32) + MariaDB + auth web PHP.
# Os binários proprietários da SoftNyx ficam em assets/ (NÃO versionado — ver README.md).
#   assets/server/        BrokenServer/ RakionWorldServ/ DB/
#   assets/web/           auth PHP (login/fetch/file) + admin/ (painel)
#   assets/wine_prefix.tgz  prefixo Wine win32 pré-construído (~393 MB)
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive \
    WINEDEBUG=-all \
    HOME=/root \
    WINEPREFIX=/root/.wine

# 32-bit + Wine + MariaDB + PHP (PDO mysql) + tooling
RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        software-properties-common wget ca-certificates gnupg2 curl \
        xvfb lib32gcc-s1 procps iproute2 winbind \
        mariadb-server mariadb-client \
        php-cli php-mysql && \
    mkdir -pm755 /etc/apt/keyrings && \
    wget -qO /etc/apt/keyrings/winehq-archive.key https://dl.winehq.org/wine-builds/winehq.key && \
    wget -qNP /etc/apt/sources.list.d/ https://dl.winehq.org/wine-builds/ubuntu/dists/jammy/winehq-jammy.sources && \
    apt-get update && \
    apt-get install -y --install-recommends winehq-stable && \
    rm -rf /var/lib/apt/lists/*

# Prefixo Wine WIN32 (binários do Rakion são 32-bit; win64/new-WoW64 falha com
# "could not load kernel32.dll"). Prefixo pré-construído pois wineboot trava em build headless.
ENV WINEARCH=win32
COPY assets/wine_prefix.tgz /tmp/wine_prefix.tgz
RUN cd /root && tar xzf /tmp/wine_prefix.tgz && rm -f /tmp/wine_prefix.tgz

# Binários do servidor + auth web embutidos na imagem (self-contained)
COPY assets/server /server
COPY assets/web /webroot

COPY entrypoint.sh /entrypoint.sh
RUN tr -d '\r' < /entrypoint.sh > /entrypoint.unix && \
    mv /entrypoint.unix /entrypoint.sh && \
    chmod +x /entrypoint.sh

# 80=auth web (Traefik), 40706=broker, 40708/tcp + 40709/udp=world
EXPOSE 80 40706 40708 40709/udp
ENTRYPOINT ["/entrypoint.sh"]
