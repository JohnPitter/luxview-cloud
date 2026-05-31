# LuxView Cloud Games — Launcher

Launcher desktop (Go + [Wails](https://wails.io)) para os jogos da LuxView Cloud. Mostra o
catálogo de servidores, baixa/instala o client, faz login e lança o jogo — substituindo
launchers proprietários (ex.: o NyxLauncher do Rakion).

## Funcionalidades

- **Catálogo** público (`GET /api/public/games`) — jogos online + "em breve" (cinza), com arte.
- **Instalar** — baixa o zip do client (`/api/public/game-client/{id}`) e extrai em
  `%APPDATA%\LuxViewLauncher\installs\<appId>`. Retry + progresso.
- **Login + launch** — login na nossa UI; lança o jogo direto (sem o launcher proprietário).
- **Opções** — resolução, modo janela/tela cheia, sensibilidade/inverter/aceleração do mouse,
  volumes e gamma (editando o `PersistentSymbols.ini`, travado read-only pra persistir).
- **Modo janela** — re-emoldura e centraliza a janela do jogo (a engine a prende no canto).
- **"Em execução"** — o botão JOGAR desabilita enquanto o jogo está aberto (checa o processo).
- Janela **frameless** com controles próprios, ícone LuxView, design system LuxView (âmbar/dark).

## Build

Requer Go, Node e a [Wails CLI](https://wails.io/docs/gettingstarted/installation).

```bash
wails dev      # desenvolvimento (hot reload)
wails build    # build de produção -> build/bin/luxview-launcher.exe
```

> O launcher embute um manifesto `requireAdministrator` (`build/windows/wails.exe.manifest`) —
> o jogo do Rakion (`rakion.bin`) exige admin, então o launcher eleva uma vez na abertura.

## Como o jogo é lançado (Rakion)

A mecânica exata (cadeia `load.bin → rakion.bin → GameGuard`, formato da linha de comando,
elevação, `config.xfs` pelo cwd, GameGuard, diálogo de modo, e persistência de opções) está
documentada na engenharia reversa em **`rakion-tutorial/LEIA-ME.md`** (seção "Launcher próprio").

Resumo no código: `launchSpec` + `Play()` em `app.go`; janela/diálogo em `windowmode_windows.go`;
registro/elevação em `launch_windows.go`.

## Configuração

- `LUXVIEW_BASE_URL` (env, opcional) — origem da plataforma (default `https://luxview.cloud`).
