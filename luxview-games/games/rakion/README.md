# Rakion (SoftNyx v258) — game template

Servidor privado de Rakion para a LuxView Cloud. Imagem **self-contained**:
broker + world (sob Wine win32) + MariaDB + auth web PHP, tudo numa imagem só.

> ⚠️ **Binários proprietários da SoftNyx não vão para o git** (`.gitignore` cobre `assets/`).
> Eles são colocados manualmente na VPS antes do build — igual ao V Rising.

## Estrutura

```
games/rakion/
├── Dockerfile          (versionado)
├── entrypoint.sh       (versionado)
├── README.md           (versionado)
├── .gitignore          (versionado) — ignora assets/
└── assets/             (NÃO versionado — só na VPS)
    ├── server/             BrokenServer/  RakionWorldServ/  DB/ (rakion_data.sql)
    ├── web/                auth PHP (config.php, launcherlogin.php, fetch/, file.php) + admin/
    └── wine_prefix.tgz     prefixo Wine win32 pré-construído (~393 MB)
```

Os `assets/` são exatamente as pastas `server/`, `web/` e `docker/wine_prefix.tgz`
do backup pessoal (`rakion-tutorial`).

## Deploy na VPS

```bash
# 1. Copiar os assets proprietários para a VPS (a partir do backup local):
scp -r rakion-tutorial/server   root@VPS:/opt/luxview-cloud/luxview-games/games/rakion/assets/server
scp -r rakion-tutorial/web      root@VPS:/opt/luxview-cloud/luxview-games/games/rakion/assets/web
scp rakion-tutorial/docker/wine_prefix.tgz \
    root@VPS:/opt/luxview-cloud/luxview-games/games/rakion/assets/wine_prefix.tgz

# 2. Build da imagem (nome esperado pelo template = luxview-cloud-rakion:latest):
cd /opt/luxview-cloud/luxview-games/games/rakion
docker build -t luxview-cloud-rakion:latest .

# 3. Rebuild da engine (para o template "rakion" aparecer no dashboard):
cd /opt/luxview-cloud && git pull && docker compose up -d --build engine dashboard
```

## Portas

| Porta        | Serviço                | Exposição                          |
|--------------|------------------------|------------------------------------|
| 40706/tcp    | BrokenServer (broker)  | publicada no host (VPS_IP:40706)   |
| 40708/tcp    | RakionWorldServ (world)| publicada no host (VPS_IP:40708)   |
| 40709/udp    | World (UDP)            | publicada no host (VPS_IP:40709)   |
| 80 (web)     | Auth + painel admin    | **Traefik → `<subdomínio>.luxview.cloud` (HTTP puro)** |

A auth web é roteada pelo Traefik via subdomínio em **HTTP puro** (sem redirect
para HTTPS) porque o NyxLauncher (2007) só fala `http://` na porta 80 e não segue
o 301→443. A engine cuida disso automaticamente (`router.go`, branch de game).

## Cliente

O `config.xfs` do cliente precisa apontar para o subdomínio da auth web. Regenere com:

```
python tools/gconfig.py <subdominio>.luxview.cloud client/Bin/config.xfs
```

(copie também para a raiz `client/`). O broker/world o cliente acessa via `VPS_IP:40706`.

## Persistência

O volume montado em `/var/lib/mysql` preserva contas e personagens entre restarts.
O dump (`rakion_data.sql`) é carregado **apenas na primeira inicialização**.

## Admin

Painel web em `https`… não — em **`http://<subdomínio>.luxview.cloud/admin/`**
(senha definida no campo "Senha do Painel Admin" do template). Cria contas, dá
Gold/Cash/itens, VIP, ban e GM.
