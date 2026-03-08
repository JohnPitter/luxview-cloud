# API Reference

Base URL: `https://luxview.cloud/api`

All authenticated endpoints require a JWT token sent as an HTTP-only cookie (set automatically after GitHub OAuth login).

## Authentication

### GitHub OAuth Login

```
GET /auth/github
```

Redirects to GitHub for OAuth authorization. After approval, GitHub redirects back to the callback URL.

### GitHub OAuth Callback

```
GET /auth/github/callback?code={code}
```

Exchanges the OAuth code for an access token, creates or updates the user, and sets a JWT cookie.

### Get Current User

```
GET /auth/me
```

Returns the authenticated user's profile, including their plan.

**Response:**

```json
{
  "id": "uuid",
  "username": "JohnPitter",
  "email": "john@example.com",
  "role": "admin",
  "avatarUrl": "https://avatars.githubusercontent.com/...",
  "plan": {
    "id": "uuid",
    "name": "Pro",
    "maxApps": 10,
    "maxCpuPerApp": 1.0,
    "maxMemoryPerApp": "2g"
  }
}
```

## Apps

### List Apps

```
GET /apps
```

Returns all apps owned by the authenticated user.

### Create App

```
POST /apps
```

**Body:**

```json
{
  "name": "my-app",
  "repoFullName": "JohnPitter/my-app",
  "branch": "main",
  "subdomain": "my-app",
  "cpuLimit": 0.5,
  "memoryLimit": "512m",
  "envVars": {
    "NODE_ENV": "production"
  }
}
```

### Get App

```
GET /apps/{id}
```

### Update App

```
PATCH /apps/{id}
```

**Body (all fields optional):**

```json
{
  "branch": "develop",
  "cpuLimit": 1.0,
  "memoryLimit": "1g",
  "autoDeploy": true,
  "envVars": {
    "NODE_ENV": "production"
  }
}
```

### Delete App

```
DELETE /apps/{id}
```

### Deploy App

```
POST /apps/{id}/deploy
```

Triggers a new deployment from the latest commit on the configured branch.

### Restart App

```
POST /apps/{id}/restart
```

### Stop App

```
POST /apps/{id}/stop
```

### Check Subdomain Availability

```
GET /apps/check-subdomain/{subdomain}
```

### Container Logs

```
GET /apps/{id}/logs?limit=100&offset=0
```

### Container Logs (SSE Stream)

```
GET /apps/{id}/logs/stream
```

Server-Sent Events stream of real-time container output.

## GitHub

### List Repositories

```
GET /github/repos
```

Returns the authenticated user's GitHub repositories.

### List Branches

```
GET /github/repos/{owner}/{repo}/branches
```

## Deployments

### List Deployments

```
GET /apps/{id}/deployments
```

### Get Deployment Logs

```
GET /deployments/{id}/logs
```

### Rollback

```
POST /deployments/{id}/rollback
```

## Services

### List All Services (across apps)

```
GET /services
```

### Add Service to App

```
POST /apps/{id}/services
```

**Body:**

```json
{
  "type": "postgres"
}
```

Valid types: `postgres`, `redis`, `mongodb`, `rabbitmq`, `s3`

### List App Services

```
GET /apps/{id}/services
```

### Delete Service

```
DELETE /services/{id}
```

## DB Explorer

### List Tables

```
GET /services/{id}/tables
```

Returns tables in the PostgreSQL database.

### Get Table Schema

```
GET /services/{id}/tables/{tableName}
```

Returns column names, types, nullable flags, and defaults.

### Execute Query

```
POST /services/{id}/query
```

**Body:**

```json
{
  "query": "SELECT * FROM users LIMIT 10"
}
```

Results are limited to 1,000 rows.

## S3 File Browser

### List Files

```
GET /services/{id}/files?prefix=folder/
```

### Upload File

```
POST /services/{id}/files/upload
```

Multipart form data. Max 50 MB per file.

### Download File

```
GET /services/{id}/files/download?key=path/to/file.txt
```

### Delete File

```
DELETE /services/{id}/files?key=path/to/file.txt
```

## Metrics

### Latest Metrics (all apps)

```
GET /apps/metrics/latest
```

### App Metrics

```
GET /apps/{id}/metrics?period=1h
```

Returns CPU, memory, and network metrics over the specified period.

## Alerts

### Create Alert

```
POST /apps/{id}/alerts
```

**Body:**

```json
{
  "metric": "cpu",
  "operator": "gt",
  "threshold": 80
}
```

### List Alerts

```
GET /apps/{id}/alerts
```

### Update Alert

```
PATCH /alerts/{id}
```

### Delete Alert

```
DELETE /alerts/{id}
```

## Plans (Public)

### List Active Plans

```
GET /plans
```

No authentication required. Returns all active plans sorted by `sort_order`. Used by the landing page pricing section.

## Admin Endpoints

All admin endpoints require the `admin` role.

### List Users

```
GET /admin/users
```

### Platform Stats

```
GET /admin/stats
```

### List All Apps

```
GET /admin/apps
```

### Update User Role

```
PATCH /admin/users/{id}/role
```

**Body:**

```json
{
  "role": "admin"
}
```

### Update App Limits

```
PATCH /admin/apps/{id}/limits
```

### Force Delete App

```
DELETE /admin/apps/{id}
```

### VPS Info

```
GET /admin/vps-info
```

### Plan Management

```
GET    /admin/plans              — List all plans (including inactive)
POST   /admin/plans              — Create plan
PATCH  /admin/plans/{id}         — Update plan
DELETE /admin/plans/{id}         — Soft delete (sets is_active=false)
PATCH  /admin/plans/{id}/default — Set as default plan
PATCH  /admin/users/{id}/plan    — Assign plan to user
```

## Webhooks

### GitHub Webhook

```
POST /webhooks/github
```

Receives push events from GitHub. Verified using the webhook signature. Triggers auto-deploy for matching apps.

## Health Check

```
GET /health
```

Returns `{"status":"ok"}`. No authentication required.

## Error Responses

All errors follow this format:

```json
{
  "error": "Description of the error"
}
```

Common HTTP status codes:

| Code | Meaning |
|---|---|
| 400 | Bad request (invalid input) |
| 401 | Unauthorized (not logged in) |
| 403 | Forbidden (plan limit reached, wrong role) |
| 404 | Not found |
| 409 | Conflict (e.g., subdomain already taken) |
| 500 | Internal server error |
