# Stage 1: get the OpenMU app files from the official image
FROM munique/openmu:latest AS openmu-source

# Stage 2: all-in-one image with PostgreSQL + OpenMU
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install PostgreSQL 16 + supervisor
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg2 supervisor \
        postgresql-16 postgresql-client-16 && \
    # .NET 10 ASP.NET runtime (matching OpenMU v0.9+)
    curl -fsSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh && \
    chmod +x /tmp/dotnet-install.sh && \
    /tmp/dotnet-install.sh --channel 10.0 --runtime aspnetcore --install-dir /usr/share/dotnet && \
    ln -s /usr/share/dotnet/dotnet /usr/bin/dotnet && \
    rm /tmp/dotnet-install.sh && \
    rm -rf /var/lib/apt/lists/*

# Copy OpenMU application from official image
COPY --from=openmu-source /app /opt/openmu

COPY entrypoint.sh /opt/entrypoint.sh
COPY supervisord.conf /etc/supervisor/conf.d/openmu.conf
RUN chmod +x /opt/entrypoint.sh

# ConnectServer (players first connect here)
EXPOSE 44405/tcp
# GameServer (actual gameplay)
EXPOSE 55901/tcp
# ChatServer (whisper/guild chat)
EXPOSE 55980/tcp
# Admin Panel (Blazor web UI)
EXPOSE 8080/tcp

ENTRYPOINT ["/opt/entrypoint.sh"]
