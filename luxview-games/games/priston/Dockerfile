# syntax=docker/dockerfile:1
# =============================================================================
# LuxView Priston Tale 4420 — OpenPriston (.NET 9)
#
# Build context MUST be the openpriston repo root:
#   docker build -t luxview-cloud-priston:latest \
#     -f luxview-games/games/priston/Dockerfile \
#     C:\Users\joaop\Desenvolvimento\openpriston
#
# The client (Field/*.smd, Char/*) is NOT in the image. Mount it at /client.
# =============================================================================
FROM mcr.microsoft.com/dotnet/sdk:9.0 AS build
WORKDIR /src
COPY Directory.Build.props ./
COPY src/OpenPriston.Protocol ./src/OpenPriston.Protocol
COPY src/OpenPriston.Server ./src/OpenPriston.Server
RUN dotnet publish src/OpenPriston.Server/OpenPriston.Server.csproj -c Release -o /app --no-self-contained

FROM mcr.microsoft.com/dotnet/aspnet:9.0
WORKDIR /app
COPY --from=build /app ./
COPY docker/luxview-entrypoint.sh /entrypoint.sh
RUN sed -i 's/\r$//' /entrypoint.sh \
    && chmod +x /entrypoint.sh \
    && mkdir -p /data/state /client /artifacts

ENV ASPNETCORE_URLS=http://0.0.0.0:5080 \
    DOTNET_EnableDiagnostics=0

EXPOSE 10012/tcp 10013/tcp 5080/tcp
ENTRYPOINT ["/entrypoint.sh"]
