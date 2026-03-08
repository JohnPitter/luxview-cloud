# Services

LuxView can provision managed services for your apps. Each service is isolated per app with dedicated credentials.

## Available Services

| Service | Description |
|---|---|
| **PostgreSQL** | Relational database (PostgreSQL 16) |
| **Redis** | In-memory cache and data store |
| **MongoDB** | Document database |
| **RabbitMQ** | Message broker (AMQP) |
| **S3 (MinIO)** | S3-compatible object storage |

## Adding a Service

1. Go to **App Detail → Services**
2. Click **Add Service**
3. Select the service type
4. Click **Create**

LuxView automatically:

1. Creates an isolated resource (database, bucket, vhost, etc.)
2. Generates a secure 24-character random password
3. Encrypts credentials at rest (AES-256-GCM)
4. Injects connection environment variables into your app on the next deploy

## Injected Environment Variables

After adding a service, these variables are automatically available in your app's container:

### PostgreSQL

```
DATABASE_URL=postgres://user:pass@host:5432/dbname
PGHOST=pg-shared
PGPORT=5432
PGUSER=<app-specific-user>
PGPASSWORD=<generated-password>
PGDATABASE=<app-specific-db>
SPRING_DATASOURCE_URL=jdbc:postgresql://host:5432/dbname
SPRING_DATASOURCE_USERNAME=<user>
SPRING_DATASOURCE_PASSWORD=<password>
```

### Redis

```
REDIS_URL=redis://:password@host:6379/0
REDIS_HOST=redis-shared
REDIS_PORT=6379
REDIS_PASSWORD=<generated-password>
```

### MongoDB

```
MONGODB_URL=mongodb://user:pass@host:27017/dbname?authSource=dbname
MONGO_URL=mongodb://user:pass@host:27017/dbname?authSource=dbname
```

### RabbitMQ

```
RABBITMQ_URL=amqp://user:pass@host:5672/vhost
AMQP_URL=amqp://user:pass@host:5672/vhost
```

### S3 (MinIO)

```
S3_ENDPOINT=http://minio-shared:9000
S3_BUCKET=<app-bucket-name>
S3_ACCESS_KEY=<generated-key>
S3_SECRET_KEY=<generated-secret>
AWS_ENDPOINT_URL=http://minio-shared:9000
AWS_ACCESS_KEY_ID=<generated-key>
AWS_SECRET_ACCESS_KEY=<generated-secret>
AWS_DEFAULT_REGION=us-east-1
```

## Service Isolation

Each service enforces strict per-app isolation:

| Service | Strategy |
|---|---|
| PostgreSQL | Dedicated database + user. `REVOKE ALL ON SCHEMA public FROM PUBLIC` prevents cross-access. |
| Redis | Unique DB number (0–15) per app |
| MongoDB | Dedicated user with `readWrite` role scoped to the app's database |
| RabbitMQ | Dedicated vhost + user with vhost-scoped permissions |
| S3 (MinIO) | Isolated bucket per app with app-specific access credentials |

## DB Explorer

For PostgreSQL services, LuxView includes a built-in database explorer:

1. Go to **App Detail → Services → click the PostgreSQL service**
2. Or navigate to the **DB Explorer** page from the sidebar

Features:

- **Table browser** — list all tables in the database
- **Schema viewer** — see columns, types, nullable flags, and defaults
- **SQL editor** — execute arbitrary queries with `Ctrl+Enter`
- **Results grid** — paginated results with copy-to-clipboard
- **Safety limit** — queries are limited to 1,000 rows

## S3 File Browser

For S3/MinIO services:

1. Go to **App Detail → Services → click the S3 service**
2. Or navigate to the **S3 Explorer** page from the sidebar

Features:

- **Folder navigation** — browse directory structures with breadcrumbs
- **Upload** — drag-and-drop or click to upload (up to 50 MB per file)
- **Download** — click any file to download
- **Delete** — remove files or folders
- **Search** — filter files by name
- **Metadata** — file size, last modified date

## Resources Page

The **Resources** page (accessible from the sidebar) shows all services across all your apps in one view. Useful for getting an overview of your provisioned infrastructure.

## Deleting a Service

1. Go to **App Detail → Services**
2. Click **Delete** on the service
3. Confirm the deletion

> **Warning:** Deleting a service permanently destroys the associated data (database, bucket, etc.). This cannot be undone.
