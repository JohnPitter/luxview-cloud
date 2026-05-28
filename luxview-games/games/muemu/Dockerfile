FROM mcr.microsoft.com/dotnet/sdk:3.1 AS build

RUN apt-get update && \
    apt-get install -y --no-install-recommends git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 https://github.com/Yomalex/MuEmu.git .
RUN dotnet publish MuEmu/MuEmu.csproj -c Release -o /app/gameserver
RUN dotnet publish CSEmu/CSEmu.csproj -c Release -o /app/connectserver

# Copy game data files
RUN cp -r Data /app/gameserver/Data

# ---

FROM mcr.microsoft.com/dotnet/aspnet:3.1

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        supervisor mysql-server mysql-client && \
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
