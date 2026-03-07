# DB Explorer + S3 Object Storage + Service Isolation

## Status: Implemented

---

## 1. Backend: DB Explorer

### Endpoints (handlers/db_explorer.go)
- `GET /api/services/{id}/tables` — list tables (PostgreSQL)
- `GET /api/services/{id}/tables/{table}` — get column schema (name, type, nullable, default)
- `POST /api/services/{id}/query` — execute arbitrary SQL (1000 row limit, 30s timeout)
- `GET /api/services/{id}/files?prefix=` — list S3 objects
- `POST /api/services/{id}/files/upload` — upload file (multipart, 50MB limit)
- `GET /api/services/{id}/files/download?key=` — download file (streams to client)
- `DELETE /api/services/{id}/files?key=` — delete S3 object

### DB Connection Logic
- Decrypts service credentials (AES-256-GCM)
- If the app has a custom `DATABASE_URL` env var, extracts the DB name from it
- Connects with provisioned user credentials but targets the actual database the app uses
- This ensures data visibility matches what the app sees while maintaining access control

## 2. S3 Object Storage (MinIO)

### Infrastructure
- MinIO container in docker-compose.yml (port 9000 data, 9001 console)
- Config: `SHARED_MINIO_HOST`, `SHARED_MINIO_PORT`, `SHARED_MINIO_USER`, `SHARED_MINIO_PASSWORD`

### Provisioning
- Service type: `s3`
- One bucket per app (`app-<uuid>`)
- Injected env vars: `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`

## 3. Service Isolation

All services enforce per-app isolation:

| Service | Isolation Mechanism |
|---|---|
| **PostgreSQL** | Dedicated DB + user with OWNER, `REVOKE ALL ON SCHEMA public FROM PUBLIC` |
| **Redis** | Unique DB number (0-15) per app via `CountByType` |
| **MongoDB** | Dedicated user with `readWrite` role scoped to app DB (via `docker exec mongosh`) |
| **RabbitMQ** | Dedicated vhost + user via Management HTTP API (port 15672) |
| **S3 (MinIO)** | Isolated bucket per app |

### Deprovisioning
- PostgreSQL: `DROP DATABASE` + `DROP USER`
- MongoDB: `dropUser` + `dropDatabase` via mongosh
- RabbitMQ: `DELETE /users` + `DELETE /vhosts` via Management API
- S3: Remove all objects + `RemoveBucket`

## 4. Frontend

### DB Explorer (`/dashboard/resources/db/:serviceId`)
- Table sidebar with list of all tables
- SQL editor with Ctrl+Enter to execute
- Results grid with column headers and row data
- Schema viewer showing column name, type, nullable, default value

### S3 File Browser (`/dashboard/resources/s3/:serviceId`)
- File listing with size, last modified, content type
- Folder navigation with breadcrumbs
- Search/filter by filename
- Upload (multi-file, 50MB limit), download, and delete actions

### Resources Page Updates
- S3 category with storage stats
- "Open Explorer" button for PostgreSQL services
- "Browse Files" button for S3 services
- S3-specific credential display (endpoint, bucket, access key, secret key)

## 5. Real-time Runtime Logs

### Backend
- `GET /api/apps/{id}/logs/stream` — SSE endpoint that streams container logs in real time
- Uses Docker `Follow` mode for continuous streaming
- Strips Docker multiplexed headers per line
- Auth via `?token=` query param (EventSource doesn't support custom headers)
- Logger middleware `responseWriter` implements `http.Flusher` for SSE compatibility

### Frontend (`RuntimeLogs` component)
- EventSource SSE connection with auto-reconnect (3s delay)
- Logs displayed **newest first** (reversed order)
- Pagination: 100 lines per page with Newer/Older navigation
- Live connection indicator (green dot + "streaming" label)
- Search/filter, auto-scroll toggle
- Color-coded log levels (error=red, warn=yellow, info=green, debug=gray)

## 6. Files Changed

### Created
- `luxview-engine/internal/api/handlers/db_explorer.go`
- `luxview-dashboard/src/pages/DbExplorer.tsx`
- `luxview-dashboard/src/pages/S3Explorer.tsx`
- `luxview-dashboard/src/components/monitoring/RuntimeLogs.tsx`

### Modified
- `luxview-engine/internal/api/router.go` — added explorer + log stream routes
- `luxview-engine/internal/api/handlers/apps.go` — SSE streaming endpoint
- `luxview-engine/internal/api/middleware/logger.go` — Flush/Unwrap for SSE
- `luxview-engine/internal/service/container.go` — LogsFollow method
- `luxview-engine/pkg/docker/client.go` — ContainerLogsFollow with Follow mode
- `luxview-engine/internal/service/provisioner.go` — S3 provisioning + isolation for all services
- `luxview-engine/internal/service/deployer.go` — env var priority (service first, user overrides)
- `luxview-engine/internal/repository/service_repo.go` — added `CountByType`
- `luxview-engine/internal/model/service.go` — added `ServiceS3` type
- `luxview-engine/Dockerfile` — added `docker-cli` for MongoDB provisioning
- `luxview-dashboard/src/api/apps.ts` — logsStreamUrl helper
- `luxview-dashboard/src/api/services.ts` — explorer API methods + S3 types
- `luxview-dashboard/src/pages/Resources.tsx` — S3 category + explorer buttons
- `luxview-dashboard/src/pages/AppDetail.tsx` — RuntimeLogs component integration
- `luxview-dashboard/src/components/services/AddServiceDialog.tsx` — S3 option
- `luxview-dashboard/src/components/services/ServiceCard.tsx` — S3 config
- `luxview-dashboard/src/App.tsx` — DB/S3 explorer routes
- `docker-compose.yml` — MinIO service
- `README.md` — full documentation update
