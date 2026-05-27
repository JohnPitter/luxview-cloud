FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV HOME=/root

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl lib32gcc-s1 python3 && \
    rm -rf /var/lib/apt/lists/*

# SteamCMD
RUN mkdir -p /opt/steamcmd && \
    curl -sqL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | \
    tar zxf - -C /opt/steamcmd

WORKDIR /opt

COPY entrypoint.sh /opt/entrypoint.sh
RUN chmod +x /opt/entrypoint.sh

# Game port (UDP) + Steam query port (UDP)
EXPOSE 27015/udp
EXPOSE 27016/udp

ENTRYPOINT ["/opt/entrypoint.sh"]
