# =============================================================================
# LuxView Tibia (Canary) — container único com MariaDB + Canary + login-server
# =============================================================================
# A imagem canary base já traz o binário canary + datapack global completo
# (data-otservbr-global) + config.lua. Este Dockerfile adiciona o MariaDB
# server e o login-server (Go static) que o client OTClient usa para login.
# =============================================================================

# login-server: binário Go estático em /bin/login-server (imagem Alpine)
FROM opentibiabr/login-server:latest AS login-server

FROM ghcr.io/opentibiabr/canary:latest

RUN apt-get update && \
    apt-get install -y --no-install-recommends mariadb-server && \
    rm -rf /var/lib/mysql && \
    rm -rf /var/lib/apt/lists/*

COPY --from=login-server /bin/login-server /usr/local/bin/login-server

COPY entrypoint.sh /opt/tibia-entrypoint.sh
COPY add_money.lua /canary/data/scripts/talkactions/god/add_money.lua
COPY ghost.lua /canary/data/scripts/talkactions/gm/ghost.lua
# O Canary global coloca todo GM em ghost mode no login. Este servidor usa GM
# visível; o comando /ghost continua disponível para alternar manualmente.
RUN sed -i 's/if player:getGroup():getId() >= GROUP_TYPE_GAMEMASTER then/if false then/' /canary/data/scripts/creaturescripts/player/login.lua
RUN sed -i 's|local createItem = TalkAction("/i")|local createItem = TalkAction("/i", "/item")|' /canary/data/scripts/talkactions/god/create_item.lua
RUN chmod +x /opt/tibia-entrypoint.sh

# login TCP, game TCP, status TCP, login HTTP (OTClient)
EXPOSE 7171/tcp 7172/tcp 7173/tcp 8088/tcp

ENTRYPOINT ["/opt/tibia-entrypoint.sh"]
