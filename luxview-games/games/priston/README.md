# Priston Tale 4220 (SunnyBPT) — template LuxView Cloud

Servidor **nativo** `SunnyBPT_v4220.exe` via Wine32 + xvfb, com MSSQL (`AGameUser`…`ZGameUser`). Não é OpenPriston .NET, Reloaded, WDPT nem Dragon.

## O que não vai para o git

- Client Windows (`SunnyBPT.exe`, `Field/`, `char/`) — copyright.
- Zip de download (`priston-4220-base.zip`) — storage global da VPS.
- Runtime do server (`GameServer/`, `Field/`, `char/`) — volume `priston-assets/server-4220`.

## Build da imagem

O contexto é esta pasta (Wine + entrypoint), não o repo OpenPriston:

```bash
docker build -t luxview-cloud-priston:latest \
  -f luxview-games/games/priston/Dockerfile \
  luxview-games/games/priston
```

Na VPS, o volume `/server` é um bind de `/data/luxview/storage/_global/priston-assets/server-4220`. O MSSQL sobe à parte como `luxview-mssql` na rede `game-net` (porta 1433 só interna). Use a `PristonSQLDll.dll` oficial (MSVC, ~40KB), nunca o stub MinGW `DSN=m2master`.

O binário importa `d3d9`/`d3dx9_43` mesmo em `*MODE SERVER`. A imagem traz Mesa i386 + Xvfb 24-bit. Em 2026-08-22 o PE ainda cai em `TopExceptionFilter` ~12s após o start e **não abre 10012** sob Wine 32 no Debian; o client zip 4220 e o launcher v1.75 já apontam para `187.77.227.65:10012`.

## Portas

| Porta     | Serviço                                      |
|-----------|----------------------------------------------|
| 10012/tcp | Game server nativo (hardcoded 0x271C)        |
| 10013/tcp | Clan (anúncio; opcional no primeiro login)   |
| 5080/tcp  | Health HTTP `/health` + register `/register` |

## Cliente

Zip base em `/data/luxview/storage/_global/priston-assets/priston-4220-base.zip`.

O launcher LuxView e a engine reescrevem `luncher.ini` (`gameServerIP` / `gameServerPORT`) e `ptReg.rgx`. O exe é **SunnyBPT.exe**, não Game.exe.

A conta é um INSERT MSSQL em `{primeira letra}GameUser` (provisionamento do launcher).

## Rates

EXP/gold/drop oficiais do 4220. Campos `PRISTON_RATE_*` existem no dashboard, mas o entrypoint **não** liga `*EVENT_EXPUP` no escuro.
