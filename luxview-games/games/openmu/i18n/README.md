# OpenMU — tradução pt-BR (satellite assemblies)

O OpenMU detecta idiomas no startup via `ResourceManager.GetResourceSet`
(`DataModel/CultureHelper.cs`), ou seja, ele só reconhece um idioma se existirem
**satellite assemblies** compiladas (`pt-BR/*.resources.dll`) ao lado das DLLs
em `/opt/openmu`. Não dá para largar `.resx`/JSON em runtime.

Este diretório gera essas satellites a partir de JSON e as embute na imagem.

## Como funciona

- **Fonte da verdade:** `json/<Short>/<ResxBase>.json` — `{chave: valor_ptBR}`.
  - `<Short>` = nome curto do assembly → assembly `MUnique.OpenMU.<Short>`.
  - `<ResxBase>` = recurso correspondente em `src/<...>/Properties/<ResxBase>.resx`
    no repositório do OpenMU.
  - Só precisa conter as chaves traduzidas; chaves ausentes caem no recurso
    neutro (inglês) automaticamente.
- `make-resx.py` converte cada JSON em `Properties/<ResxBase>.pt-BR.resx`.
- `build-satellites.sh` cria um projeto stub resource-only por assembly, com
  `AssemblyName`/versão casando a imagem oficial (`0.9.9.0`, sem strong name),
  e compila `out/pt-BR/MUnique.OpenMU.<Short>.resources.dll`.
- O `Dockerfile` do openmu roda isso num stage `i18n` (dotnet SDK 10 + python3)
  e sobrepõe `out/pt-BR` em `/opt/openmu/pt-BR`.

## Escopo atual

Toda a UI que um humano lê: AdminPanel, Web.Shared, Web.Map, GameLogic
(PlayerMessage), Network, DataModel/Resources, Persistence.Initialization,
Interfaces/ModelResources. **~650 strings.**

Fora de escopo (caem em inglês, baixa visibilidade): metadados técnicos em massa
— `DataModel/ModelResources` (1903 captions de campo), `GameServer` e
`GameLogic/PlugInResources` (descrições de handlers de pacote, ~1055). Para
incluí-los, basta adicionar os JSON correspondentes em `json/<Short>/`.

## Build manual / teste local

```bash
cd luxview-games/games/openmu/i18n
bash build-satellites.sh   # requer dotnet SDK 10 + python3
ls out/pt-BR
```
