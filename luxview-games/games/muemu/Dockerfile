# Stage 1: build MuEmu from source
FROM ubuntu:22.04 AS build

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl git libicu70 && \
    # Install .NET 6 SDK (can build netcoreapp3.1 targets with roll-forward)
    curl -fsSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && \
    chmod +x /tmp/dotnet-install.sh && \
    /tmp/dotnet-install.sh --channel 6.0 --install-dir /usr/share/dotnet && \
    ln -s /usr/share/dotnet/dotnet /usr/bin/dotnet && \
    rm /tmp/dotnet-install.sh && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 https://github.com/Yomalex/MuEmu.git .

# Build with roll-forward to work around the missing 3.1 targeting pack
RUN dotnet publish MuEmu/MuEmu.csproj -c Release -o /app/gameserver \
      /p:TargetFramework=net6.0 || \
    dotnet publish MuEmu/MuEmu.csproj -c Release -o /app/gameserver

RUN dotnet publish CSEmu/CSEmu.csproj -c Release -o /app/connectserver \
      /p:TargetFramework=net6.0 || \
    dotnet publish CSEmu/CSEmu.csproj -c Release -o /app/connectserver

# Copy game data files
RUN cp -r Data /app/gameserver/Data

# Stage 2: runtime
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl supervisor \
        mysql-server mysql-client && \
    # .NET 6 runtime
    curl -fsSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && \
    chmod +x /tmp/dotnet-install.sh && \
    /tmp/dotnet-install.sh --channel 6.0 --runtime aspnetcore --install-dir /usr/share/dotnet && \
    ln -s /usr/share/dotnet/dotnet /usr/bin/dotnet && \
    rm /tmp/dotnet-install.sh && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /app/gameserver /opt/muemu/gameserver
COPY --from=build /app/connectserver /opt/muemu/connectserver

COPY entrypoint.sh /opt/entrypoint.sh
COPY supervisord.conf /etc/supervisor/conf.d/muemu.conf
RUN chmod +x /opt/entrypoint.sh

# ConnectServer (players first connect here)
EXPOSE 44405/tcp
# GameServer (actual gameplay)
EXPOSE 55901/tcp

ENTRYPOINT ["/opt/entrypoint.sh"]
