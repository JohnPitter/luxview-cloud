# LuxView Cloud independente do GitHub

## Objetivo

Tornar o LuxView Cloud a fonte primaria do ciclo de vida dos repositorios. O usuario ainda pode autenticar com GitHub, mas GitHub deixa de ser dependencia operacional para criar app, rodar Actions, fazer deploy e gerenciar codigo.

GitHub passa a ser apenas um remote opcional de backup/mirror dos repositorios hospedados no LuxView.

## Estado atual

- Auth usa GitHub OAuth.
- Apps apontam para `repo_url` e `repo_branch`.
- Deploy clona o repositorio externo via token GitHub.
- Auto-deploy depende de webhook GitHub.
- LuxView Actions ja executa internamente, mas ainda clona o repositorio externo.
- GitHub App cria repositorios, commita workflow e sincroniza secrets no GitHub.

## Arquitetura alvo

1. GitHub OAuth continua somente como identidade/login.
2. LuxView cria e hospeda repositorios proprios em storage persistente.
3. Cada app referencia um repositorio interno por `repository_id`.
4. Deploy e Actions fazem checkout a partir do repositorio LuxView.
5. Push para LuxView dispara evento interno:
   - identifica branch e commit
   - roda LuxView Actions
   - se passar e `auto_deploy=true`, enfileira deploy
6. GitHub vira backup opcional:
   - LuxView configura remote de backup
   - LuxView executa push para GitHub
   - falha de backup nao bloqueia deploy por padrao

## Modelo de dados proposto

### `repositories`

- `id UUID PRIMARY KEY`
- `user_id UUID NOT NULL REFERENCES users(id)`
- `name VARCHAR(100) NOT NULL`
- `slug VARCHAR(120) NOT NULL`
- `default_branch VARCHAR(100) NOT NULL DEFAULT 'main'`
- `storage_path TEXT NOT NULL`
- `visibility VARCHAR(20) NOT NULL DEFAULT 'private'`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `UNIQUE(user_id, slug)`

### `repository_remotes`

- `id UUID PRIMARY KEY`
- `repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE`
- `provider VARCHAR(40) NOT NULL`
- `remote_url TEXT NOT NULL`
- `mode VARCHAR(20) NOT NULL DEFAULT 'backup'`
- `last_sync_at TIMESTAMPTZ`
- `last_sync_status VARCHAR(20)`
- `last_sync_error TEXT`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `UNIQUE(repository_id, provider, remote_url)`

### Ajuste em `apps`

- Adicionar `repository_id UUID REFERENCES repositories(id)`.
- Manter `repo_url` e `repo_branch` durante migracao.
- Futuramente trocar `repo_branch` por `branch` ou manter como compatibilidade API.

## Servicos internos

### `RepositoryService`

Fonte unica para operacoes de repositorio:

- `Create(ctx, userID, name, defaultBranch)`
- `ImportFromGitHub(ctx, user, owner, repo, branch)`
- `Checkout(ctx, repositoryID, ref, destDir)`
- `ResolveRef(ctx, repositoryID, branch)`
- `ListBranches(ctx, repositoryID)`
- `ConfigureBackupRemote(ctx, repositoryID, provider, remoteURL)`
- `SyncBackup(ctx, repositoryID, remoteID)`

### `SourceCheckout`

Interface para desacoplar deploy/actions da origem do codigo:

```go
type SourceCheckout interface {
    Checkout(ctx context.Context, app *model.App, ref string, destDir string) (*CheckoutResult, error)
}
```

O primeiro corte pode ter duas implementacoes:

- `GitHubCheckout`: compatibilidade com fluxo atual.
- `LuxViewRepositoryCheckout`: novo fluxo interno.

## Fluxos alvo

### Criar app novo

1. Usuario cria repo LuxView vazio ou importa de GitHub.
2. LuxView cria `repositories`.
3. Usuario configura app apontando para `repository_id` e branch.
4. LuxView roda analise/build/deploy a partir do checkout interno.

### Push para LuxView

1. Usuario faz push para `https://git.luxview.cloud/{user}/{repo}.git`.
2. Hook interno registra branch/commit.
3. LuxView procura apps que usam esse `repository_id` e branch.
4. LuxView roda Actions se houver workflow.
5. Se Actions passar ou nao houver workflow, deploy entra na fila.
6. Backup para GitHub roda em background se configurado.

### Backup GitHub

1. Usuario conecta GitHub App/OAuth.
2. LuxView cria ou seleciona repositorio de backup.
3. LuxView adiciona remote no bare repo interno.
4. A cada push interno, LuxView faz `git push` para o remote de backup.
5. UI mostra ultimo status de backup.

## Branches, commits e pull requests

Branches e commits fazem parte do escopo obrigatorio do novo fluxo. Sem isso, LuxView nao vira uma fonte primaria real para os repositorios.

O primeiro corte deve suportar:

- Push e pull Git normal.
- Branches por repositorio.
- Default branch.
- Historico basico de commits.
- Deploy a partir de uma branch configurada no app.
- Actions executadas por branch/ref.

Pull requests internos devem ser preparados no modelo, mas nao precisam entrar no primeiro corte funcional.

Fluxo alvo de pull request:

1. Usuario cria uma branch, por exemplo `feature-x`.
2. Usuario faz push para LuxView.
3. LuxView roda Actions nessa branch.
4. Usuario abre PR `feature-x -> main`.
5. LuxView mostra diff, commits, checks e logs.
6. Reviewer aprova ou solicita ajuste.
7. LuxView faz merge.
8. Merge na branch configurada do app dispara Actions.
9. Se Actions passar, deploy automatico entra na fila.
10. Backup GitHub sincroniza depois.

Tabelas futuras provaveis:

- `pull_requests`
- `pull_request_reviews`
- `pull_request_comments`
- `pull_request_checks`
- caches opcionais para branches/commits, se necessario para performance

Importante: Git continua sendo a fonte primaria para branches, commits e tags. O banco deve guardar metadados, cache e estado de produto, nao duplicar o repositorio como fonte de verdade.

## Fases de implementacao

### Fase 1: base de repositorios internos

- Criar migrations para `repositories` e `repository_remotes`.
- Criar modelos e repository layer.
- Criar `RepositoryService`.
- Adicionar `repository_id` em `apps`.
- Suportar default branch, listagem de branches e resolucao de commit por branch/ref.
- Testes funcionais para create, branch list, resolve ref e checkout.

Criterio: e possivel criar repo interno, listar branches, resolver commit e fazer checkout local por service.

### Fase 2: abstrair origem do deploy

- Introduzir `SourceCheckout`.
- Fazer `Deployer` usar a interface em vez de chamar `RepoCloner` diretamente.
- Fazer `ActionService` usar a mesma interface.
- Manter GitHub como provider legado.

Criterio: apps atuais continuam deployando via GitHub sem regressao.

### Fase 3: deploy a partir de repo LuxView

- Implementar `LuxViewRepositoryCheckout`.
- Permitir criar app com `repository_id`.
- Resolver commit SHA internamente.
- Ajustar logs/deployments para source `luxview`.

Criterio: app criado a partir de repo LuxView faz build e deploy sem chamada ao GitHub.

### Fase 4: endpoint Git para push

- Escolher transporte inicial:
  - Git HTTP smart protocol via service dedicado.
  - SSH Git server com forced command.
- Implementar autenticacao por token LuxView.
- Implementar post-receive hook interno.
- Enfileirar Actions/deploy por evento interno.

Criterio: `git push` para LuxView dispara Actions/deploy.

### Fase 5: UI principal independente

- Trocar wizard de novo app:
  - criar repo LuxView
  - importar de GitHub
  - selecionar repo LuxView existente
- GitHub deixa de ser primeira etapa obrigatoria.
- Atualizar pagina de Actions para listar workflows/actions cadastrados no repositorio.
- Cada workflow deve exibir nome, path, eventos configurados, branch/ref alvo, ultimo status, ultimo run e acoes disponiveis.
- Botao de executar deve ficar associado a um workflow especifico, nao ser apenas uma acao global em uma pagina vazia.
- Atualizar textos de produto, docs e tours.

Criterio: usuario cria app completo sem listar repositorios GitHub.
Criterio adicional: usuario abre a aba Actions, ve os workflows cadastrados e executa um workflow especifico manualmente.

### Fase 6: GitHub como backup

- Adaptar GitHub App service para operacoes de mirror/backup.
- Criar endpoints para configurar/sincronizar backup.
- Mostrar ultimo sync na UI.
- Garantir que falha de backup nao interrompe deploy.

Criterio: repo LuxView sincroniza para GitHub sob demanda e em background apos push.

## Compatibilidade de Actions

No curto prazo, manter suporte a `.github/workflows/*.yml` para projetos importados.

Adicionar como caminho preferencial:

- `.luxview/workflows/*.yml`

Ordem de busca sugerida:

1. `.luxview/workflows/*.yml`
2. `.github/workflows/*.yml`

## Riscos e decisoes abertas

- Transporte Git: HTTP e mais facil de integrar com navegador/reverse proxy; SSH costuma ser melhor para experiencia Git tradicional.
- Multi-tenant storage: paths precisam ser derivados de IDs internos, nunca de input do usuario.
- Isolamento de Actions: execucao atual usa Docker com limites basicos; precisa endurecer antes de aceitar codigo arbitrario de repos internos.
- Backup remoto: definir se push para GitHub e por branch normal ou mirror completo.
- Migracao: apps existentes com `repo_url` devem continuar funcionando ate serem importados para repo LuxView.

## Primeiro PR recomendado

Implementar somente o corte estrutural:

1. Migrations e modelos de repositorio interno.
2. `RepositoryService` minimo.
3. Interface `SourceCheckout`.
4. Adapter GitHub legado usando comportamento atual.
5. Trocar `Deployer` e `ActionService` para dependerem da interface.

Esse PR nao muda UI nem comportamento visivel. Ele cria a base para substituir GitHub sem quebrar o fluxo atual.
