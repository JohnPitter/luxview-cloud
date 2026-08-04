# Metin2 Legacy — template de jogo

Este template representa o servidor legado que já foi validado com o client
atual do OpenMetin. O servidor novo em .NET não faz parte deste fluxo.

O engine do LuxView inicia um único container por servidor. Por isso, a imagem
usada pelo template precisa ser uma imagem self-contained que contenha os
binários legados, o banco MySQL/MariaDB e um entrypoint que suba o `db`, o
`auth`, os quatro cores do canal 1 e o `game99`. O compose de desenvolvimento
do OpenMetin, com `db` e `server` separados, continua sendo a referência local;
ele não deve ser apontado diretamente como imagem do template.

## Contrato da imagem

Nome esperado: `luxview-cloud-metin2-legacy:latest`.

Portas publicadas pelo template:

| Porta | Uso |
|---:|---|
| 11000/tcp | Auth legado |
| 13001/tcp | Canal 1 / core principal |
| 13002/tcp | Canal 1 / core 2 |
| 13003/tcp | Canal 1 / core 3 |
| 13004/tcp | Canal 1 / core 4 |
| 13099/tcp | Game 99 |

A imagem deve ler `LUXVIEW_PUBLIC_IP` para anunciar o IP público nos arquivos
`CONFIG` e aceitar `METIN_DB_USER` e `METIN_DB_PASSWORD`. O volume
`/var/lib/mysql` é persistente por servidor.

## Client base

Monte um zip contendo a pasta funcional
`Metin2FullClient/` na raiz do arquivo, incluindo:

- `Metin2Distribute.exe`;
- `pack/root.data`;
- `locale.cfg` e os demais arquivos do client.

Coloque o artefato no storage da plataforma:

```text
/data/luxview/storage/metin2-legacy-assets/metin2-legacy-client-base.zip
```

O engine gera uma cópia por servidor, substituindo o IP legado no `root.data`,
as portas `AUTH_PORT`, `CH1_PORT` e `MARKADDR`, e forçando `1252 pt` no
`locale.cfg`. O client continua fazendo login dentro do próprio jogo, como no
launcher legado funcional.

## Publicação no launcher

Depois de criar o servidor pelo template, abra a configuração do jogo e marque
`Exibir no launcher LuxView = Sim` (`LUXVIEW_LISTED=true`). O card só fica
clicável quando o servidor estiver online e o client base estiver disponível.
