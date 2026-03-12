# Email Hosting Feature — Plano de Implementação

## Objetivo
Permitir que cada app no LuxView Cloud tenha mailboxes de email (`user@appname.luxview.cloud`) com webmail integrado, controlado pelos limites do plano.

## Arquitetura

### Stack
- **docker-mailserver** — Postfix (SMTP) + Dovecot (IMAP) + Rspamd (anti-spam) em um container compartilhado
- **Roundcube** — Webmail leve, acessível em `mail.luxview.cloud`
- **Traefik** — Roteamento HTTPS para webmail
- **Engine** — CRUD de mailboxes via `docker exec` no container de email

### Modelo de Dados
- Novo `ServiceType = "email"` no enum
- Cada app pode ter **1 serviço de email**
- Dentro do serviço, múltiplas **mailboxes** (tabela separada)
- Credentials do serviço: configurações SMTP/IMAP genéricas
- Cada mailbox: `address@appname.luxview.cloud` + password

### Acesso do Usuário
- **Webmail**: `mail.luxview.cloud` → Roundcube → login com `user@app.luxview.cloud`
- **IMAP/SMTP**: clientes externos (Outlook, Thunderbird) com credenciais
- **Gestão**: painel no dashboard para criar/excluir mailboxes

### DNS (luxview.cloud)
```
MX     *.luxview.cloud    → mail.luxview.cloud (priority 10)
A      mail.luxview.cloud → IP_VPS
TXT    luxview.cloud      → "v=spf1 a mx ip4:IP_VPS ~all"
TXT    _dmarc.luxview.cloud → "v=DMARC1; p=quarantine; rua=mailto:dmarc@luxview.cloud"
DKIM   → gerado pelo docker-mailserver, adicionado ao DNS
```

### Limites por Plano
- `MaxMailboxesPerApp int` — quantas mailboxes o app pode criar
- `MaxMailboxStorage string` — storage por mailbox (ex: "500m", "1g")

---

## Fases de Implementação

### Fase A — Infraestrutura (docker-compose + DNS)

**Arquivos:**

| Arquivo | Ação | Descrição |
|---------|------|-----------|
| `docker-compose.yml` | Modificar | Adicionar containers `mailserver` e `roundcube` |
| DNS (Cloudflare/registrar) | Manual | MX, SPF, DKIM, DMARC records |
| `traefik/dynamic/routes.yml` | Modificar | Rota para `mail.luxview.cloud` → Roundcube |

**docker-compose adições:**
```yaml
  mailserver:
    image: ghcr.io/docker-mailserver/docker-mailserver:latest
    container_name: luxview-mailserver
    hostname: mail.luxview.cloud
    ports:
      - "25:25"     # SMTP
      - "465:465"   # SMTPS
      - "587:587"   # Submission
      - "993:993"   # IMAPS
    volumes:
      - mail-data:/var/mail
      - mail-state:/var/mail-state
      - mail-logs:/var/log/mail
      - mail-config:/tmp/docker-mailserver
      - /etc/localtime:/etc/localtime:ro
    environment:
      - ENABLE_RSPAMD=1
      - ENABLE_CLAMAV=0           # economiza RAM
      - ENABLE_FAIL2BAN=1
      - SSL_TYPE=letsencrypt
      - PERMIT_DOCKER=connected-networks
      - POSTMASTER_ADDRESS=admin@luxview.cloud
      - OVERRIDE_HOSTNAME=mail.luxview.cloud
    networks:
      - luxview-net
    restart: unless-stopped

  roundcube:
    image: roundcube/roundcubemail:latest
    container_name: luxview-roundcube
    environment:
      - ROUNDCUBEMAIL_DEFAULT_HOST=ssl://mailserver
      - ROUNDCUBEMAIL_DEFAULT_PORT=993
      - ROUNDCUBEMAIL_SMTP_SERVER=tls://mailserver
      - ROUNDCUBEMAIL_SMTP_PORT=587
      - ROUNDCUBEMAIL_UPLOAD_MAX_FILESIZE=25M
    volumes:
      - roundcube-data:/var/roundcube/db
    depends_on:
      - mailserver
    networks:
      - luxview-net
    restart: unless-stopped

volumes:
  mail-data:
  mail-state:
  mail-logs:
  mail-config:
  roundcube-data:
```

**Traefik route para webmail:**
```yaml
  roundcube:
    rule: "Host(`mail.luxview.cloud`)"
    service: roundcube-svc
    tls:
      certResolver: le

  roundcube-svc:
    loadBalancer:
      servers:
        - url: "http://luxview-roundcube:80"
```

**Verificação:** Container mailserver rodando, Roundcube acessível em `mail.luxview.cloud`, DKIM key gerado.

---

### Fase B — Backend: Model + Migration + Provisioner

**Arquivos:**

| Arquivo | Ação | Descrição |
|---------|------|-----------|
| `internal/model/service.go` | Modificar | Adicionar `ServiceEmail` |
| `internal/model/plan.go` | Modificar | Adicionar `MaxMailboxesPerApp`, `MaxMailboxStorage` |
| `internal/model/mailbox.go` | Criar | Struct `Mailbox` (id, serviceID, address, createdAt) |
| `migrations/012_email_service.sql` | Criar | ALTER TYPE + CREATE TABLE mailboxes |
| `internal/repository/mailbox_repo.go` | Criar | CRUD para mailboxes |
| `internal/service/provisioner.go` | Modificar | Case `ServiceEmail` em Provision/Deprovision |

**Migration 012:**
```sql
ALTER TYPE service_type ADD VALUE IF NOT EXISTS 'email';

ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_mailboxes_per_app INTEGER NOT NULL DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_mailbox_storage VARCHAR(20) NOT NULL DEFAULT '500m';

CREATE TABLE IF NOT EXISTS mailboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES app_services(id) ON DELETE CASCADE,
    address VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mailboxes_service_id ON mailboxes(service_id);
```

**Model mailbox.go:**
```go
type Mailbox struct {
    ID        uuid.UUID `json:"id"`
    ServiceID uuid.UUID `json:"service_id"`
    Address   string    `json:"address"`
    CreatedAt time.Time `json:"created_at"`
}
```

**Provisioner — email case:**
- `Provision`: cria o service com credentials `{smtp_host, smtp_port, imap_host, imap_port, domain}`
- Não cria mailboxes automaticamente (o usuário faz via API)
- `Deprovision`: remove todas as mailboxes do docker-mailserver + deleta o service

**Verificação:** Migration roda, service type email aceito, provisioning funciona.

---

### Fase C — Backend: API de Mailboxes

**Arquivos:**

| Arquivo | Ação | Descrição |
|---------|------|-----------|
| `internal/api/handlers/mailbox.go` | Criar | Handlers: List, Create, Delete mailboxes |
| `internal/api/router.go` | Modificar | Adicionar rotas de mailbox |
| `internal/api/handlers/services.go` | Modificar | Aceitar `email` como service type válido |

**Rotas:**
```
GET    /services/{id}/mailboxes        → ListMailboxes
POST   /services/{id}/mailboxes        → CreateMailbox
DELETE /mailboxes/{id}                  → DeleteMailbox
POST   /mailboxes/{id}/reset-password  → ResetMailboxPassword
```

**CreateMailbox handler:**
1. Validar ownership (service → app → user)
2. Verificar limite do plano (`MaxMailboxesPerApp`)
3. Gerar endereço: `{local}@{appSubdomain}.luxview.cloud`
4. Gerar password aleatória
5. Executar `docker exec mailserver setup email add {address} {password}`
6. Aplicar quota: `docker exec mailserver setup quota set {address} {maxStorage}`
7. Salvar no DB (mailboxes table)
8. Retornar address + password (uma única vez)

**DeleteMailbox handler:**
1. Executar `docker exec mailserver setup email del {address}`
2. Remover do DB

**Verificação:** Criar/listar/excluir mailboxes via API funciona, email enviado/recebido.

---

### Fase D — Frontend: UI de Email

**Arquivos:**

| Arquivo | Ação | Descrição |
|---------|------|-----------|
| `src/api/services.ts` | Modificar | Adicionar types + API calls para mailboxes |
| `src/pages/EmailManager.tsx` | Criar | Página de gestão de mailboxes |
| `src/pages/Resources.tsx` | Modificar | Card de email com link para gestão |
| `src/components/services/ServiceCard.tsx` | Modificar | Layout específico para email |
| `src/App.tsx` | Modificar | Rota `/dashboard/resources/email/:serviceId` |
| `src/i18n/locales/*.json` | Modificar | Keys de email |

**EmailManager.tsx:**
- Header com nome do app + link para webmail
- Lista de mailboxes (address, criado em)
- Botão criar mailbox (dialog com campo "local part")
- Password exibida uma única vez no dialog de criação
- Botão resetar password
- Botão excluir mailbox
- Indicador de uso: X/Y mailboxes (limite do plano)

**ServiceCard para email:**
- Ícone de email (Mail icon)
- Domínio: `@appname.luxview.cloud`
- Contagem de mailboxes
- Link para webmail (`mail.luxview.cloud`)
- Botão "Gerenciar" → EmailManager

**Verificação:** Criar mailbox pelo dashboard, acessar webmail, enviar/receber email.

---

## Dependências Críticas

1. **DNS**: MX record para `*.luxview.cloud` → IP da VPS (sem isso, nada funciona)
2. **Porta 25**: Muitos provedores de VPS bloqueiam porta 25 por padrão — verificar com DigitalOcean
3. **SSL**: docker-mailserver precisa do certificado Let's Encrypt (compartilhar com Traefik ou gerar separado)
4. **RAM**: docker-mailserver + Roundcube usam ~500MB extras

## Riscos

| Risco | Mitigação |
|-------|-----------|
| Porta 25 bloqueada na VPS | Abrir ticket com provedor; ou usar relay externo |
| IP em blacklist | Monitorar via mxtoolbox; warm-up gradual |
| Spam enviado por usuários | Rspamd + rate limiting por mailbox |
| SSL compartilhado | Montar volume de certs do Traefik no mailserver |
| Storage crescente | Quotas por mailbox enforced pelo Dovecot |

## Ordem de Implementação
Fase A → B → C → D (sequencial, cada fase depende da anterior)
