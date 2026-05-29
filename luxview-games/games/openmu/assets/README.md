# OpenMU — Client base zip

A engine gera o client de download de cada servidor a partir de um **zip base**
(`openmu-s6-base.zip`) montado neste diretório.

Fluxo (`handlers.DownloadClient` → `service.WriteOpenMUClientZip`):

1. Jogador clica em **Baixar Client** no dashboard do servidor.
2. A engine abre `openmu-s6-base.zip`.
3. Remove o `launcher.config` original e gera um novo apontando para o
   `Address`/`Port` daquele servidor (IP da VPS + `gamePort`, normalmente 44405).
4. Faz stream do zip já configurado para o jogador.

O `launcher.config` é XML (`XmlSerializer` do `LauncherSettings`):
`MainExePath` + `Hosts[].{Description,Address,Port}`. As resoluções caem no
default do launcher quando ausentes — não precisamos incluí-las.

## Como montar o `openmu-s6-base.zip`

O client do MU é copyright e tem ~1–2 GB, por isso **não fica versionado no git**
(ver `.gitignore`). O único passo que exige um humano é obter o client em si —
o OpenMU **não** distribui o client e não há download livre; pegue na FAQ do
Discord oficial: <https://discord.gg/2u5Agkd> (MU Online Season 6 Episode 3 EN).

Com a pasta do client em mãos, o `build-base-zip.sh` faz o resto (baixa o
ClientLauncher oficial, junta tudo e empacota no path montado pela engine):

```bash
cd /opt/luxview-cloud/luxview-games/games/openmu/assets
./build-base-zip.sh /caminho/para/o/client-s6
```

> O `launcher.config` fica na **raiz** do zip — é o nome exato que a engine
> procura para substituir (`EqualFold("launcher.config")`).

### Requisito no lado do jogador

O ClientLauncher v0.9.6 precisa do **.NET 10 Desktop Runtime** no Windows do
jogador: <https://dotnet.microsoft.com/download/dotnet/10.0>

## Onde colocar na VPS

O binário **não fica no repositório**. Ele vive no volume do serviço
**"Armazenamento"** (storage) da plataforma, sob `STORAGE_BASE_PATH`
(`/data/luxview/storage/app-<id>/`), que já é montado na engine.

A engine lê o caminho de `OPENMU_CLIENT_BASE_ZIP` (definido no
`docker-compose.yml`/`.env` apontando para esse volume). Para atualizar o
client, suba o novo `openmu-s6-base.zip` pelo Storage Explorer do dashboard,
ou copie direto na VPS:

```bash
scp -i ~/.ssh/vps_key openmu-s6-base.zip \
  root@187.77.227.65:/data/luxview/storage/app-<id-do-storage>/openmu-s6-base.zip
```

Sem esse arquivo o endpoint responde `404 — OpenMU client base zip not found`
e o botão de download mostra erro (degrada de forma controlada).

> O diretório `assets/` deste repo serve só de área de staging para montar o
> zip com o `build-base-zip.sh` — o artefato final vai para o storage.
