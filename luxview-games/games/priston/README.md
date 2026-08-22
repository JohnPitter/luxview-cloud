# Priston Tale 4420 (OpenPriston) — template LuxView Cloud

Servidor .NET 9 do repositório `openpriston`, rebrandado para **LuxView** (no lugar de BPT / SunnyBPT). Drops oficiais do catálogo 4220 com taxas configuráveis; mobs G1–G3 e F1–F4 e NPCs de Ricarten/Pillai alinhados ao mapa clássico (PristonWorld EPT).

## O que não vai para o git

- Client Windows (`game.exe`, `Field/`, `Char/`) — copyright.
- Zip de download (`priston-4420-base.zip`) — storage global da VPS.

## Build da imagem

O contexto do Docker é a raiz do **openpriston**, não deste repositório:

```bash
docker build -t luxview-cloud-priston:latest \
  -f luxview-games/games/priston/Dockerfile \
  C:/Users/joaop/Desenvolvimento/openpriston
```

Na VPS:

```bash
# código do servidor (sem client)
scp -r -i ~/.ssh/vps_key C:/Users/joaop/Desenvolvimento/openpriston \
  root@187.77.227.65:/opt/openpriston

# meshes/modelos do client 4420/5421 (Field + Char)
scp -r -i ~/.ssh/vps_key \
  "C:/Users/joaop/Desenvolvimento/openpriston/Priston Tale Brasil Reloaded (Beta)/Field" \
  "C:/Users/joaop/Desenvolvimento/openpriston/Priston Tale Brasil Reloaded (Beta)/Char" \
  root@187.77.227.65:/data/luxview/storage/_global/priston-assets/client/

docker build -t luxview-cloud-priston:latest \
  -f /opt/luxview-cloud/luxview-games/games/priston/Dockerfile \
  /opt/openpriston
```

O volume `/client` do app é um bind read-only de `/data/luxview/storage/_global/priston-assets/client` (`Field/` e `Char/` na raiz). O estado (contas, personagens) fica no volume `/data/state`.

## Portas

| Porta     | Serviço                          |
|-----------|----------------------------------|
| 10012/tcp | Gateway do jogo (login + mundo)  |
| 10013/tcp | Clan (anúncio no server list)    |
| 5080/tcp  | Health HTTP `/health`            |

## Cliente

Zip base em `/data/luxview/storage/_global/priston-assets/priston-4420-base.zip`. No download, a engine reescreve `ptReg.rgx` e `openpriston.launcher.ini` com o IP público e o nome **LuxView**.

A conta é criada no primeiro login (arquivo JSON no volume `/data/state`). O launcher LuxView não usa SQL Server.

## Rates padrão

- EXP 5x, ouro 3x, drop 2x (campos `PRISTON_RATE_*` no dashboard).
