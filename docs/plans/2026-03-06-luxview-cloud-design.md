# LuxView Cloud Platform — Design Document

> Data: 2026-03-06
> Status: Draft

## 1. Visao Geral

LuxView Cloud e uma plataforma PaaS (Platform as a Service) hospedada em VPS propria com dominio `luxview.cloud`. Permite que qualquer usuario conecte seu GitHub, selecione um repositorio e faca deploy com um clique. A plataforma detecta automaticamente a tecnologia, builda, roda e configura subdominios SSL automaticamente.

**Exemplo:** usuario deploya o repo "agenthub" e acessa em `agenthub.luxview.cloud`.

## 2. Decisoes de Projeto

| Aspecto | Decisao |
|---|---|
| VPS | Linux limpo (Ubuntu 22.04 LTS) |
| Publico | Multi-tenant aberto (qualquer pessoa cria conta) |
| Escopo de apps | Full stack + servicos extras (DB, Redis, filas, etc.) |
| Monetizacao | Depois — foco no produto primeiro |
| Deploy flow | GitHub OAuth → selecionar repo → clica "Deploy" |
| Build | Buildpacks automaticos (zero config) + Dockerfile se existir |
| Isolamento | Docker containers com cgroups (CPU/RAM/disk limits) |
| Servicos (DB, Redis) | Compartilhados com isolamento logico (database/vhost por app) |
| Monitoramento | Avancado (logs, metricas, alertas, uptime) |
| Subdominio | Nome do app (slug unico global, first-come first-served) |
| Dominio custom | Futuro — usuario aponta CNAME para luxview.cloud |
| Stack da plataforma | Go (Engine) + React/TypeScript (Dashboard) |

## 3. Arquitetura de Alto Nivel

```
+-------------------------------------------------------+
|                    luxview.cloud                        |
|                                                         |
|  +---------+   +----------+   +--------------------+   |
|  | Traefik |-->| LuxView  |-->|  Docker Engine      |   |
|  | (Proxy) |   | Engine   |   |                      |   |
|  |         |   | (Go)     |   |  [A1] [A2] [A3]     |   |
|  | :80/443 |   | :8080    |   |   user containers    |   |
|  +---------+   +----------+   +--------------------+   |
|       |                                                  |
|       |         +----------+   +--------------------+   |
|       +-------->| Dashboard|   | Shared Services      |   |
|                 | (React)  |   | PostgreSQL :5432     |   |
|                 | :3000    |   | Redis      :6379     |   |
|                 +----------+   | MongoDB    :27017    |   |
|                                | RabbitMQ   :5672     |   |
|                                +--------------------+   |
+-------------------------------------------------------+
```

### Componentes

1. **Traefik (Reverse Proxy)** — Recebe todo trafego em :80/:443. Roteia `app.luxview.cloud` para o container certo, `luxview.cloud` para o dashboard. SSL automatico via Let's Encrypt. Configuracao dinamica via HTTP provider do Engine.

2. **LuxView Engine (Go)** — O cerebro. API REST + WebSocket. Gerencia builds, deploys, containers, portas, DNS, health checks, metricas. Comunica com Docker Engine via SDK.

3. **Dashboard (React)** — SPA onde o usuario gerencia tudo. Conecta repos, monitora apps, ve logs, configura variaveis de ambiente.

4. **Shared Services** — PostgreSQL, Redis, MongoDB, RabbitMQ compartilhados. Cada usuario recebe database/vhost isolado, criados automaticamente pelo Engine.

## 4. Data Model

```sql
-- Usuarios (via GitHub OAuth)
users
  id              UUID PK
  github_id       BIGINT UNIQUE
  username        VARCHAR(100)
  email           VARCHAR(255)
  avatar_url      TEXT
  github_token    TEXT (encrypted AES-256-GCM)
  role            ENUM('user', 'admin')
  created_at      TIMESTAMP
  last_login_at   TIMESTAMP

-- Apps deployadas
apps
  id              UUID PK
  user_id         UUID FK -> users
  name            VARCHAR(100)
  subdomain       VARCHAR(100) UNIQUE  -- slug global unico
  repo_url        TEXT
  repo_branch     VARCHAR(100) DEFAULT 'main'
  stack           VARCHAR(50)  -- 'node', 'python', 'go', 'static', 'docker'
  status          ENUM('building', 'running', 'stopped', 'error', 'sleeping')
  container_id    VARCHAR(100)
  internal_port   INT
  assigned_port   INT UNIQUE
  env_vars        JSONB (encrypted)
  resource_limits JSONB  -- {cpu: "0.5", memory: "512m", disk: "1g"}
  auto_deploy     BOOLEAN DEFAULT true
  created_at      TIMESTAMP
  updated_at      TIMESTAMP

-- Servicos provisionados por app
app_services
  id              UUID PK
  app_id          UUID FK -> apps
  service_type    ENUM('postgres', 'redis', 'mongodb', 'rabbitmq')
  db_name         VARCHAR(100)
  credentials     JSONB (encrypted)
  created_at      TIMESTAMP

-- Historico de deploys
deployments
  id              UUID PK
  app_id          UUID FK -> apps
  commit_sha      VARCHAR(40)
  commit_message  TEXT
  status          ENUM('pending', 'building', 'deploying', 'live', 'failed', 'rolled_back')
  build_log       TEXT
  duration_ms     INT
  image_tag       VARCHAR(255)
  created_at      TIMESTAMP
  finished_at     TIMESTAMP

-- Metricas (time-series simplificado)
metrics
  id              BIGSERIAL PK
  app_id          UUID FK -> apps
  cpu_percent     FLOAT
  memory_bytes    BIGINT
  network_rx      BIGINT
  network_tx      BIGINT
  timestamp       TIMESTAMP (index)

-- Alertas
alerts
  id              UUID PK
  app_id          UUID FK -> apps
  metric          VARCHAR(50)
  condition       VARCHAR(20)
  threshold       FLOAT
  channel         ENUM('email', 'webhook', 'discord')
  channel_config  JSONB
  enabled         BOOLEAN DEFAULT true
  last_triggered  TIMESTAMP
```

### Indices

- `apps(user_id)` — listar apps do usuario
- `apps(subdomain)` — lookup rapido pelo Traefik
- `apps(assigned_port)` — evitar colisao de portas
- `deployments(app_id, created_at DESC)` — historico por app
- `metrics(app_id, timestamp DESC)` — queries de metricas por periodo

### Campos encriptados

`github_token`, `env_vars`, `credentials` — AES-256-GCM com chave em variavel de ambiente.

## 5. Deploy Flow

```
GitHub OAuth    Clone Repo    Detect Stack    Build Image    Start Container
  + Select  -->            -->              -->            -->  + Route
   (user)       (Engine)      (Engine)        (Engine)        (Engine)
```

### Passo a passo

1. **Usuario seleciona repo** — Dashboard lista repos via GitHub API. Usuario escolhe repo, branch, e nome do subdominio (validado como unico).

2. **Clone** — Engine clona o repo em `/tmp/builds/<deploy-id>/`. Shallow clone (`--depth 1`).

3. **Deteccao de stack** — Engine analisa arquivos raiz:

| Arquivo encontrado | Stack detectada | Buildpack |
|---|---|---|
| `package.json` + `next.config.*` | Next.js | Node 20 + `npm run build` + `npm start` |
| `package.json` + `vite.config.*` | Vite/React/Vue | Node 20 + `npm run build` -> Nginx static |
| `package.json` (generico) | Node.js | Node 20 + `npm start` |
| `requirements.txt` / `pyproject.toml` | Python | Python 3.12 + `pip install` + `gunicorn` |
| `go.mod` | Go | Go 1.22 + `go build` |
| `Cargo.toml` | Rust | Rust + `cargo build --release` |
| `Dockerfile` | Custom | Usa o Dockerfile direto (prioridade maxima) |
| `index.html` (so estatico) | Static | Nginx servindo os arquivos |

4. **Build** — Engine gera Dockerfile baseado no buildpack (ou usa existente), executa `docker build -t luxview/<app-name>:<commit-sha>`. Logs streamed via WebSocket.

5. **Start** — Porta disponivel no range 10000-65000, `docker run` com resource limits, env vars injetadas. Rota registrada no Traefik: `<subdomain>.luxview.cloud -> localhost:<porta>`.

6. **Health check** — GET a cada 5s por 60s. Se 200 -> `running`. Se timeout -> `error`.

7. **Auto-deploy** — Webhook do GitHub notifica a cada push. Blue-green: sobe novo container, valida health, remove antigo.

## 6. Roteamento Dinamico (Traefik)

DNS: wildcard record `*.luxview.cloud -> IP da VPS`.

Traefik usa HTTP provider que consulta o Engine a cada 5s:

```
GET /api/internal/traefik-config

Response:
{
  "http": {
    "routers": {
      "app-agenthub": {
        "rule": "Host(`agenthub.luxview.cloud`)",
        "service": "app-agenthub",
        "tls": { "certResolver": "letsencrypt" }
      }
    },
    "services": {
      "app-agenthub": {
        "loadBalancer": {
          "servers": [{ "url": "http://host.docker.internal:10042" }]
        }
      }
    }
  }
}
```

SSL automatico por subdominio via Let's Encrypt.

## 7. Provisioning de Servicos Compartilhados

Quando usuario adiciona PostgreSQL a sua app:

1. Conecta no `pg-shared` como `luxview_admin`
2. `CREATE DATABASE app_<app_id>; CREATE USER app_<app_id>_user WITH PASSWORD '<random>';`
3. Grants apenas nesse database
4. Credenciais encriptadas salvas em `app_services`
5. Env var injetada no container: `DATABASE_URL=postgres://app_xxx_user:pass@pg-shared:5432/app_xxx`

Mesmo padrao para Redis (database number), MongoDB (database), RabbitMQ (vhost).

## 8. Estrutura do Codigo — Engine (Go)

```
luxview-engine/
  cmd/
    engine/
      main.go                    # Entrypoint
  internal/
    api/
      router.go                  # Monta rotas (Gin/Chi)
      middleware/
        auth.go                  # JWT validation
        ratelimit.go             # Rate limiting
        logger.go                # Request logging
      handlers/
        auth.go                  # GitHub OAuth
        apps.go                  # CRUD apps + deploy
        deployments.go           # Historico, rollback
        services.go              # Provisionar DB/Redis
        metrics.go               # Metricas por app
        alerts.go                # CRUD alertas
        admin.go                 # Admin endpoints
    service/
      deployer.go                # Orquestra deploy flow
      builder.go                 # Gera Dockerfile, docker build
      detector.go                # Detecta stack
      container.go               # Start/stop/restart containers
      portmanager.go             # Aloca/libera portas
      router.go                  # Registra rotas no Traefik
      provisioner.go             # Cria databases/vhosts
      healthcheck.go             # Health monitoring
      metrics_collector.go       # Coleta docker stats
      alerter.go                 # Avalia regras de alerta
      github.go                  # GitHub API client
      webhook.go                 # Processa push events
    buildpack/
      buildpack.go               # Interface comum
      node.go                    # Node.js / Next.js / Vite
      python.go                  # Python / Django / FastAPI
      golang.go                  # Go
      rust.go                    # Rust
      static.go                  # HTML/CSS/JS -> Nginx
      dockerfile.go              # Usa Dockerfile do repo
    model/
      user.go
      app.go
      deployment.go
      service.go
      metric.go
      alert.go
    repository/
      user_repo.go
      app_repo.go
      deployment_repo.go
      service_repo.go
      metric_repo.go
      alert_repo.go
    worker/
      build_worker.go            # Fila de builds (goroutine pool)
      metrics_worker.go          # Coleta metricas a cada 30s
      healthcheck_worker.go      # Health checks a cada 15s
      cleanup_worker.go          # Remove builds/images antigos
      alert_worker.go            # Avalia alertas a cada 60s
    config/
      config.go                  # Env vars + defaults
  pkg/
    crypto/
      aes.go                     # AES-256-GCM encrypt/decrypt
    docker/
      client.go                  # Docker SDK wrapper
    logger/
      logger.go                  # zerolog structured logger
  migrations/
    001_create_users.sql
    002_create_apps.sql
    003_create_deployments.sql
    004_create_app_services.sql
    005_create_metrics.sql
    006_create_alerts.sql
  Dockerfile
  docker-compose.yml
  go.mod
  .env.example
```

### Design patterns

- **Handler -> Service -> Repository** (3 camadas)
- **Buildpacks como interface** — facil adicionar novas stacks
- **Worker pool** — goroutines de longa duracao com graceful shutdown
- **Build queue** — max 3 builds simultaneos (configuravel)

## 9. Estrutura do Codigo — Dashboard (React)

```
luxview-dashboard/
  src/
    main.tsx
    App.tsx
    api/
      client.ts                  # Axios + interceptors
      auth.ts
      apps.ts
      deployments.ts
      services.ts
      metrics.ts
      alerts.ts
      github.ts
    stores/
      auth.store.ts
      apps.store.ts
      notifications.store.ts
    hooks/
      useWebSocket.ts
      useDeployLogs.ts
      useMetricsLive.ts
    pages/
      Landing.tsx                # Hero, features, "Login with GitHub"
      Login.tsx                  # OAuth redirect
      Dashboard.tsx              # Grid de AppCards + stats
      NewApp.tsx                 # Wizard: repo -> branch -> subdomain -> env -> deploy
      AppDetail.tsx              # Tabs: overview, deploys, logs, env, services, metrics, alerts, settings
      Admin.tsx                  # Painel admin
    components/
      layout/
        Sidebar.tsx
        Header.tsx
        MainLayout.tsx
      apps/
        AppCard.tsx
        AppStatusBadge.tsx
        RepoSelector.tsx
        SubdomainInput.tsx
      deploy/
        DeployWizard.tsx
        BuildLogViewer.tsx
        DeployHistory.tsx
      monitoring/
        LogViewer.tsx
        MetricsChart.tsx
        UptimeBar.tsx
        AlertConfig.tsx
      services/
        ServiceCard.tsx
        AddServiceDialog.tsx
      common/
        Terminal.tsx
        StatusDot.tsx
        EmptyState.tsx
        ConfirmDialog.tsx
    lib/
      websocket.ts
      format.ts
    styles/
      globals.css
  public/
    logo.svg
  index.html
  vite.config.ts
  tailwind.config.ts
  package.json
```

### Telas principais

- **Dashboard** — Grid de AppCards (nome, subdominio, status, stack, ultimo deploy, CPU/RAM). Filtros por status. Botao "New App".
- **NewApp** — Wizard 4 steps: selecionar repo -> branch + subdominio -> env vars -> review + deploy.
- **AppDetail** — Tabs: Overview, Deployments, Logs, Environment, Services, Metrics, Alerts, Settings.

### Tech choices

React 18, Vite, Tailwind CSS, Zustand, Recharts, Lucide icons.

## 10. Infraestrutura (docker-compose)

Setup completo da VPS em um unico `docker-compose.yml`:

- **traefik** — Reverse proxy, SSL, roteamento dinamico
- **engine** — Go API (monta Docker socket)
- **dashboard** — React SPA (Nginx)
- **pg-platform** — PostgreSQL da plataforma
- **pg-shared** — PostgreSQL compartilhado dos usuarios
- **redis-shared** — Redis compartilhado
- **mongo-shared** — MongoDB compartilhado
- **rabbitmq-shared** — RabbitMQ compartilhado

### Networks

- `luxview-net` — comunicacao interna (engine <-> databases)
- `traefik-public` — trafego externo (traefik <-> containers dos usuarios)

### Volumes persistentes

- `pg-platform-data`, `pg-shared-data`, `redis-shared-data`, `mongo-shared-data`, `rabbitmq-shared-data`
- `traefik-certs` — certificados SSL
- `build-cache` — cache de Docker layers

### Environment vars requeridas

- `DB_PASSWORD` — senha do pg-platform
- `ENCRYPTION_KEY` — chave AES-256 para campos encriptados
- `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` — OAuth app
- `JWT_SECRET` — assinatura dos tokens
- `SHARED_PG_PASSWORD`, `SHARED_REDIS_PASSWORD`, `SHARED_MONGO_PASSWORD`, `SHARED_RABBITMQ_PASSWORD`

## 11. Roadmap de Implementacao

### Fase 1 — Setup da VPS e Infraestrutura Base

- Provisionar VPS (Ubuntu 22.04 LTS)
- Instalar Docker Engine + Docker Compose
- Firewall (UFW): portas 22, 80, 443
- DNS: `luxview.cloud` + `*.luxview.cloud` -> IP da VPS
- Subir docker-compose com Traefik + pg-platform + pg-shared + redis-shared
- Validar wildcard SSL funcionando

**Criterio:** wildcard SSL ok, Traefik roteando, banco acessivel.

### Fase 2 — Engine Base (Go API)

**2A — Scaffolding**
- Projeto Go, estrutura de pastas, config loader, logger, PostgreSQL + migrations

**2B — Auth**
- GitHub OAuth flow, JWT, auth middleware
- Endpoints: `/api/auth/github`, `/api/auth/github/callback`, `/api/auth/me`

**2C — Apps CRUD**
- CRUD completo de apps, validacao de subdomain, lista de subdominios reservados

**2D — GitHub Integration**
- Listar repos/branches via GitHub API, webhook receiver

**Criterio:** criar app via API com user autenticado via GitHub.

### Fase 3 — Deploy Flow

**3A — Stack Detection + Buildpacks**
- Interface Buildpack, implementar Node.js, Python, Go, Static, Dockerfile

**3B — Build Pipeline**
- Clone, detectar stack, gerar Dockerfile, docker build com streaming de logs

**3C — Container Management**
- Port Manager, docker run com resource limits, health check loop

**3D — Routing Dinamico**
- Endpoint traefik-config, SSL automatico, blue-green deploy

**3E — Webhooks (Auto-deploy)**
- Registrar webhook no GitHub, processar push events, auto-redeploy

**Criterio:** push no GitHub -> app rebuilda automaticamente com zero downtime.

### Fase 4 — Dashboard MVP

**4A — Setup + Auth**
- React + Vite + Tailwind + Zustand, landing page, GitHub OAuth, layout base

**4B — Apps + Deploy**
- Dashboard grid, NewApp wizard, build log viewer, AppDetail

**4C — Management**
- Env vars editor, deploy history + rollback, app settings, confirm dialogs

**Criterio:** usuario faz login, seleciona repo, deploya e ve app rodando pela UI.

### Fase 5 — Servicos Compartilhados

**5A — Provisioning Engine**
- Criar database/user/vhost, credenciais encriptadas, injecao de env vars, cleanup

**5B — Dashboard UI**
- Tab Services, Add Service dialog, ServiceCard com credenciais

**Criterio:** adicionar PostgreSQL e `DATABASE_URL` aparecer automaticamente.

### Fase 6 — Monitoring + Alerts

**6A — Metricas**
- Collector worker (docker stats a cada 30s), API de metricas agregadas, graficos Recharts

**6B — Logs**
- docker logs via WebSocket, LogViewer com search e filtro

**6C — Alerts + Uptime**
- Alert rules, alert worker, canais (email, webhook, Discord), uptime tracker

**Criterio:** receber alerta quando app cai, ver graficos, buscar nos logs.

### Fase 7 — Polish + Scale

**7A — Seguranca**
- Rate limiting, validacao de input, audit log, limites por user, CORS

**7B — UX Polish**
- Empty states, skeleton loaders, toasts, responsive, error boundaries

**7C — Admin Panel**
- Listar users/apps, stats globais, force stop/delete, ajustar limites

**7D — Dominios Customizados (futuro)**
- CNAME validation, rota no Traefik, Let's Encrypt para dominio custom

### Dependencias entre Fases

| Fase | Depende de |
|---|---|
| 1 | Nenhuma |
| 2 | Fase 1 |
| 3 | Fase 2 |
| 4 | Fase 2 (API pronta) |
| 5 | Fase 3 |
| 6 | Fase 3 + 4 |
| 7 | Todas anteriores |

> Fases 3 e 4 podem ter trabalho parcialmente paralelo (backend + frontend).
