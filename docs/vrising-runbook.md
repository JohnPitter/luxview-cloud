# V Rising / Games Companion Runbook

## Contexto

O servidor V Rising roda no container `luxview-vrising`.
O painel de controle roda no container `luxview-games-companion`.

Arquivos e volumes importantes:

- Compose remoto: `/opt/luxview-cloud/docker-compose.yml`
- Volume de dados: `luxview-cloud_vrising-data`
- Config persistida: `/vrising-data/server-config.env`
- Config do jogo: `/vrising-data/Settings/ServerGameSettings.json`
- Container do jogo: `luxview-vrising`
- Container da UI: `luxview-games-companion`

Host SSH usado hoje:

```sh
ssh vps
```

## Incidente: loop de restart depois de salvar config

Sintoma na UI:

- Estado ficava `restarting`
- Contador de reinicializações subia
- Uptime reiniciava sempre

Comando que revelou a causa:

```sh
ssh vps "docker logs --tail 160 luxview-vrising 2>&1"
```

Erro visto:

```txt
/vrising-data/server-config.env: line 1: Gamers: command not found
/vrising-data/server-config.env: line 1: Rising: command not found
```

Causa:

O `vrising/entrypoint.sh` faz `source /vrising-data/server-config.env`.
O `games-companion` gravava valores sem aspas, por exemplo:

```sh
VRISING_SERVER_NAME=Pseudo Gamers
```

Como havia espaço, o shell interpretava `Gamers` como comando.
O processo saía com código `127` e o Docker reiniciava o container.

Formato correto:

```sh
VRISING_SERVER_NAME='Pseudo Gamers'
VRISING_DESCRIPTION='Um Servidor de Amigos'
VRISING_PASSWORD='125125'
```

Hotfix usado no volume:

```sh
ssh vps "docker run --rm -i -v luxview-cloud_vrising-data:/data alpine sh -c 'cat > /data/server-config.env'"
```

Depois reiniciar:

```sh
ssh vps "docker restart luxview-vrising"
```

Validação:

```sh
ssh vps "docker inspect luxview-vrising --format 'State={{.State.Status}} Exit={{.State.ExitCode}} Restarts={{.RestartCount}}'"
ssh vps "docker logs --since 2m luxview-vrising 2>&1 | grep -E 'command not found|Startup Completed|Server Setup Complete|Exception|Error' || true"
```

Estado esperado:

```txt
State=running Exit=0 Restarts=0
```

## Fix permanente: quoting do server-config.env

O `games-companion` agora grava todos os valores como string shell-safe.
Também lê de volta removendo as aspas para preencher a UI.

Pontos de código:

- `games-companion/main.go`: `shellQuote`
- `games-companion/main.go`: `parseShellValue`
- `games-companion/main.go`: `saveConfig`
- `games-companion/main.go`: `loadConfig`
- `games-companion/main_test.go`: teste para valor com espaço e apóstrofo

Teste local:

```sh
cd games-companion
go test ./...
go build ./...
```

## Incidente: jogadores online apareciam como vazio ou 0/0

Sintoma:

- A UI mostrava jogadores online como `-`
- O servidor estava online
- O protocolo A2S respondia corretamente quando testado manualmente

Teste manual feito na VPS:

```sh
ssh vps "python3 - <<'PY'
import socket, binascii
addr=('127.0.0.1',27016)
s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)
s.settimeout(3)
req=b'\xff\xff\xff\xffTSource Engine Query\x00'
s.sendto(req, addr)
data,_=s.recvfrom(1400)
if len(data)>=9 and data[4]==0x41:
    s.sendto(req+data[5:9], addr)
    data,_=s.recvfrom(1400)
print(len(data), binascii.hexlify(data[:40]), data[:120])
PY"
```

Resposta real indicava jogadores:

```txt
players=3 max_players=40
```

Causa:

Depois do challenge A2S, o código fazia `buf = buf[:n]` com `n=9`.
Na segunda leitura, `conn.Read(buf)` lia no máximo 9 bytes porque o slice continuava curto.
O parser recebia resposta truncada e retornava `0/0`.

Correção:

Antes da segunda leitura:

```go
buf = buf[:cap(buf)]
```

Teste adicionado:

- `TestQueryA2SHandlesChallengeResponse`

## Rede Docker para query A2S

O `games-companion` precisa consultar o V Rising pela porta UDP `27016`.
Para reduzir dependência da porta publicada no host, ele foi colocado também na `game-net`.

Compose esperado:

```yaml
games-companion:
  networks:
    - traefik-public
    - game-net
```

Ordem de query no código:

1. `luxview-vrising:27016`
2. `host.docker.internal:27016`

Validar redes do companion:

```sh
ssh vps "docker inspect luxview-games-companion --format '{{json .NetworkSettings.Networks}}'"
```

Deve aparecer `game-net` e `traefik-public`.

## Recursos do servidor na UI

O endpoint de status agora retorna:

- CPU %
- memória usada
- limite de memória
- rede recebida
- rede enviada
- quantidade de processos

Validação autenticada usada na VPS:

```sh
ssh vps "python3 - <<'PY'
import subprocess, urllib.parse, urllib.request, http.cookiejar, json
inspect = json.loads(subprocess.check_output(['docker','inspect','luxview-games-companion'], text=True))[0]
ip = inspect['NetworkSettings']['Networks']['traefik-public']['IPAddress']
pw = 'admin'
for item in inspect['Config']['Env']:
    if item.startswith('MANAGER_PASSWORD='):
        pw = item.split('=',1)[1]
base = 'http://%s:8080' % ip
cj = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))
opener.open(urllib.request.Request(base + '/login', data=urllib.parse.urlencode({'password': pw}).encode(), method='POST'), timeout=5)
data = json.load(opener.open(base + '/api/servers/vrising/status', timeout=8))
print(json.dumps({k:data.get(k) for k in ('running','players','max_players','player_query_error','resources','resource_error')}, indent=2))
PY"
```

Resultado esperado:

```json
{
  "running": true,
  "players": 3,
  "max_players": 40,
  "player_query_error": null,
  "resources": {
    "memory_usage": "3.0 GiB",
    "memory_limit": "15.6 GiB",
    "pids": 113
  },
  "resource_error": null
}
```

CPU e rede variam a cada leitura.

## Deploy cirúrgico do Games Companion

Enviar arquivos:

```sh
scp docker-compose.yml vps:/opt/luxview-cloud/docker-compose.yml
scp games-companion/main.go games-companion/main_test.go games-companion/go.mod games-companion/go.sum vps:/opt/luxview-cloud/games-companion/
```

Rebuild apenas do companion:

```sh
ssh vps "cd /opt/luxview-cloud && docker compose up -d --build games-companion"
```

Validar:

```sh
ssh vps "docker ps --format 'table {{.Names}}\t{{.Status}}' | grep -E 'luxview-vrising|luxview-games-companion|NAMES'"
ssh vps "docker logs --tail 30 luxview-games-companion 2>&1"
ssh vps "docker inspect luxview-vrising --format 'State={{.State.Status}} Exit={{.State.ExitCode}} Restarts={{.RestartCount}}'"
```

## Comandos rápidos úteis

Logs do V Rising:

```sh
ssh vps "docker logs --tail 160 luxview-vrising 2>&1"
```

Stats pontuais:

```sh
ssh vps "docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}' luxview-vrising luxview-games-companion"
```

Ver config persistida:

```sh
ssh vps "docker run --rm -v luxview-cloud_vrising-data:/data alpine sh -c 'nl -ba /data/server-config.env'"
```

Testar se o arquivo pode ser sourced:

```sh
ssh vps "docker run --rm -v luxview-cloud_vrising-data:/data alpine sh -c '. /data/server-config.env; printf \"%s|%s\n\" \"$VRISING_SERVER_NAME\" \"$VRISING_DESCRIPTION\"'"
```

Ver volumes montados:

```sh
ssh vps "docker inspect luxview-vrising --format '{{range .Mounts}}{{.Name}} {{.Destination}}{{println}}{{end}}'"
```
