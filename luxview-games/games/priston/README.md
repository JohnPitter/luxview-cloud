# Priston Tale 4220 (SunnyBPT) — template LuxView Cloud

Runtime nativo `SunnyBPT_v4220.exe` via **Ubuntu 24.04 + Wine 9.0 (Wine32) + Xvfb**, reproduzindo exatamente o ambiente funcional de `legacy-docker`. O volume `/server` continua sendo o bind `priston-assets/server-4220`; a imagem contém apenas o ambiente de execução. Requisitos de build: Docker BuildKit, `vendor/MDAC_TYP.EXE` e os CABs D3DX9; o host de execução precisa fornecer as redes/volumes da stack LuxView.

## Requisitos descobertos (produção)

- O banco precisa ser um **SQL Server 2017 dedicado** (`priston-sql`, preferencialmente em container). O SQL Server 2019 rejeita o handshake TLS 1.0 usado pelo DBNETLIB (`SECDoClientHandshake`).
- Distribua a `PristonSQLDll.dll` **adaptada** presente na pasta-fonte, junto do DSN `m2master` e do registro `OpenPriston\\SqlAdapter`. A DLL oficial falha silenciosamente em `SQLLoginProcess`/`SQLLogoutProcess`.
- `*GAME_SERVER` em `hotuk.ini` deve anunciar o **IP público nos três campos**. O cliente reconecta aos endpoints anunciados; anunciar o IP interno do Docker trava após a seleção.
- Ambiente validado: Ubuntu 24.04 + Wine 9.0 (Wine32).
- Configure `pids-limit: 4096` no container do jogo.

## Build

```bash
docker build -t luxview-cloud-priston:latest \
  -f luxview-games/games/priston/Dockerfile luxview-games/games/priston
```

O build não baixa componentes Windows: `vendor/` contém MDAC_TYP.EXE e os CABs oficiais D3DX9. `bake-prefix.sh` extrai o MDAC 2.8 SP1 manualmente, instala SQLOLEDB/ADO/ODBC, registra componentes e typelibs (inclusive ADO 2.7) e cria o DSN `m2master` com o driver SQL Server. O `Dockerfile` instala `mssql-tools18` pelo repositório Microsoft Noble; se a Microsoft ainda não publicar o pacote para 24.04, o mesmo passo faz fallback documentado para o repositório Ubuntu 22.04.

## Fluxo de boot

1. Aguarda `PRISTON_MSSQL_HOST` (padrão `luxview-mssql`) e inicia `socat` em `127.0.0.1:1433`. O relay é o caminho principal porque o `sql.dll` oficial usa esse Data Source hardcoded; `PRISTON_MSSQL_PORT` permite MSSQL em outra porta.
2. Aplica `init-accountdb.sql` com `sqlcmd` de forma idempotente. Cria `accountdb` e os catálogos vazios BillingDb, BillingLogDb, GameLogDb, PCRoom, PCRoomLog, ItemLogDb, ClanDb e Sod2Db. Falha de schema é registrada em `/artifacts/schema.log` sem impedir o boot.
3. Gera `SunnyBPT_docker.exe` como cópia LARGEADDRESSAWARE (bit PE `0x20`), preservando o original, e executa a cópia.
4. Importa um único `.reg` com todas as conexões dos bancos e valida a chave GameServer. A conta HTTP permanece em `:5080` (`/health`, `/register`, `/players`).
5. Inicia Xvfb `:99` em 24-bit e o servidor nas portas 10012/10013.

`patch_sql_dll.py` continua disponível como fallback opcional: defina `PRISTON_PATCH_SQL_DLL=1`; normalmente não é necessário e não deve ser usado no lugar do relay.

## Contrato LuxView

A engine injeta `PRISTON_SERVER_NAME`, `PRISTON_PUBLIC_IP`, `PRISTON_MSSQL_HOST`, `PRISTON_MSSQL_PORT` e `PRISTON_MSSQL_PASSWORD`. A imagem é `luxview-cloud-priston:latest`, o volume é montado em `/server`, e as portas são 10012 (jogo), 10013 (clan) e 5080 (contas/health).

## Arquivos fora do git

Client Windows, zip de download e conteúdo do servidor (`GameServer/`, `Field/`, `char/`) permanecem no storage global/volume. Use a `PristonSQLDll.dll` oficial e os DLLs nativos do servidor; rates não são alteradas pelo entrypoint.
