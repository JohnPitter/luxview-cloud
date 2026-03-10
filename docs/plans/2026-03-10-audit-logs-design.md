# Audit Logs — Design

## Objetivo
Registrar todas as operações CRUD de todos os usuários com diff de valores (old/new), visualizável pelo admin em tabela e timeline.

## Schema

```sql
-- 008_create_audit_logs.sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_username VARCHAR(100) NOT NULL,
    action VARCHAR(20) NOT NULL,
    resource_type VARCHAR(30) NOT NULL,
    resource_id VARCHAR(100),
    resource_name VARCHAR(200),
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
```

## Backend

### AuditLogRepo
- `Create(ctx, log)` — insert event
- `List(ctx, filters, limit, offset)` — query paginada com filtros
- `Count(ctx, filters)` — total para paginação
- `DeleteOlderThan(ctx, cutoff)` — cleanup (90 dias)

### AuditService
- `Log(ctx, AuditEntry)` — fire-and-forget em goroutine, nunca bloqueia request
- Extrai actor do context, simplifica chamada nos handlers

### AuditEntry struct
```go
type AuditEntry struct {
    ActorID       uuid.UUID
    ActorUsername string
    Action        string   // "create", "update", "delete", "deploy", "login"
    ResourceType  string   // "app", "user", "service", "plan", "setting", "deployment", "alert"
    ResourceID    string
    ResourceName  string
    OldValues     any
    NewValues     any
    IPAddress     string
}
```

### Endpoints (admin only)
- `GET /admin/audit-logs` — lista paginada
  - Query params: limit, offset, actor_id, action, resource_type, resource_id, from, to, search
- `GET /admin/audit-logs/stats` — contadores 24h por ação e recurso

### Handlers auditados (~20 endpoints)
- App: create, update, delete, deploy, restart, stop
- Service: create, delete
- Deployment: rollback
- User: role change, plan assign
- Settings: AI update, cleanup update, cleanup trigger
- Plan: create, update, delete, set default
- Admin: force delete app, update app limits
- Auth: login (GitHub callback)

## Frontend

### Tab "Audit" no Admin.tsx
- Toggle entre Table e Timeline
- Badge 24h no header

### Tabela
- Colunas: Hora, Usuário, Ação, Recurso, Detalhes
- Badges coloridos por ação (create=verde, update=azul, delete=vermelho, deploy=purple, login=zinc)
- Expandir para ver diff (old → new)
- Filtros: action, resource_type, search, date range
- Paginação

### Timeline
- Feed vertical agrupado por dia
- Avatar + ícone ação + texto descritivo
- Hover expande diff e IP
- Ícones por resource_type

### Cleanup
- cleanup_worker deleta audit_logs > 90 dias
