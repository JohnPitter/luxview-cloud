# LuxView MU Launcher

Launcher customizado para o client OpenMU: painel de notícias + auto-update +
botão JOGAR. Substitui o `MUnique.OpenMU.ClientLauncher.exe` simples.

## Como funciona

- **Conexão:** reusa a lógica do OpenMU (`Launcher.cs`, MIT) — grava IP/porta em
  `HKLM\SOFTWARE\WebZen\Mu\Connection` (encodado) e abre `main.exe` com `/u /p`.
  Por isso o `app.manifest` pede `requireAdministrator` (write no HKLM).
- **Servidor:** lido do `launcher.config` (XML do OpenMU) que a engine personaliza
  por servidor (nome / IP / porta / caminho do main.exe).
- **Notícias + patch:** `luxview-launcher.json` (na raiz do client) aponta:
  - `newsUrl` → página exibida no WebView2 (notícias do servidor)
  - `patchManifestUrl` → manifesto de update (JSON)
- **Auto-update:** baixa o manifesto, compara SHA-256 de cada arquivo local e
  baixa só os que mudaram (com barra de progresso). Falha de rede não bloqueia
  o jogo.

### Formato do manifesto de patch

```json
{
  "version": "1",
  "files": [
    { "path": "Data/Local/Eng/Text.bmd", "sha256": "<HEX>", "url": "https://mu.luxview.cloud/patch/Data/Local/Eng/Text.bmd" }
  ]
}
```
`url` é opcional — se ausente, usa `<base do manifesto>/patch/<path>`. Manifesto
vazio (`files: []`) = "cliente atualizado".

## Requisitos no PC do jogador

- **WebView2 Runtime** (vem no Windows 11 e na maioria dos Win10 com Edge; se
  faltar, o painel de notícias some mas o launcher continua funcionando).
- O launcher precisa rodar como administrador (UAC) para escrever o IP no registro.

## Build (cross-compile no Linux)

```bash
dotnet publish -c Release -r win-x64 --self-contained false -o out
```
Gera `out/LuxViewLauncher.exe` (+ dlls do WebView2). É empacotado no
`openmu-s6-base.zip` que a engine serve.
