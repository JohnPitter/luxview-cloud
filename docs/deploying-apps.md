# Deploying Apps

## Creating an App

1. Click **New App** from the dashboard
2. Select a GitHub repository from the list
3. Choose a branch (default: `main`)
4. Set a subdomain — availability is checked in real time
5. Optionally configure CPU and memory limits
6. Click **Create & Deploy**

## Environment Variables

Add environment variables in the app settings. They are:

- **Encrypted at rest** using AES-256-GCM
- **Injected** into your container on every deploy
- **Hidden** in the UI after saving (for security)

Common variables:

```
NODE_ENV=production
PORT=3000
DATABASE_URL=postgres://...    # Auto-injected if you add a PostgreSQL service
```

Service credentials (DATABASE_URL, REDIS_URL, etc.) are automatically injected when you attach a service to your app. See [Services](./services.md).

## Build Process

When you trigger a deploy, LuxView:

1. **Clones** your repository at the specified branch/commit
2. **Detects** the stack (Node.js, Python, Go, etc.)
3. **Generates** a Dockerfile if your project doesn't have one (buildpack system)
4. **Builds** a Docker image
5. **Decrypts** environment variables and injects them + service credentials
6. **Starts** an isolated container with resource limits
7. **Runs** health checks (HTTP GET on the app's port)
8. **Updates** Traefik routing to point the subdomain to the new container
9. **Removes** the old container (if this is a redeploy)

### Build Logs

Every deployment captures full build logs. View them from:

- **App Detail → Deployments tab** → click any deployment → view logs

### Build Concurrency

The platform supports concurrent builds (default: 3). If more builds are queued than the concurrency limit, they wait in a FIFO queue.

## Auto Deploy

When auto-deploy is enabled for an app:

1. LuxView registers a GitHub webhook on your repository
2. Every push to the configured branch triggers a new deployment automatically
3. The webhook payload is verified using GitHub's signature mechanism

Enable auto-deploy from **App Settings → Auto Deploy toggle**.

> Auto-deploy requires the `auto_deploy_enabled` permission on your plan.

## Manual Deploy

Click **Deploy** on the app detail page to trigger a new deployment at any time. This clones the latest commit from the configured branch.

## Rollback

Every successful deployment is saved. To rollback:

1. Go to **App Detail → Deployments**
2. Find the deployment you want to restore
3. Click **Rollback**

This redeploys the exact image from that previous deployment — no rebuild needed.

## App Lifecycle

### Start / Stop / Restart

From the app detail page:

- **Stop** — stops the container (app goes offline)
- **Start** — starts a stopped container
- **Restart** — stops and starts the container

### Delete

Deleting an app:

1. Stops and removes the container
2. Removes the Traefik routing entry
3. **Does not** delete attached services — delete them separately if needed

## Resource Limits

Each app can have CPU and memory limits:

| Setting | Default | Description |
|---|---|---|
| CPU | 0.25 cores | Max CPU allocation (Docker `--cpus`) |
| Memory | 512 MB | Max memory allocation (Docker `--memory`) |
| Disk | 1 GB | Max disk usage |

These are enforced by Docker cgroups. If your app exceeds the memory limit, the container is killed and restarted.

Plan-based limits may apply — see [Admin Guide](./admin-guide.md) for plan configuration.

## Monitoring

### Real-time Metrics

The app detail page shows live metrics:

- **CPU usage** — percentage of allocated CPU
- **Memory usage** — current vs. limit
- **Network I/O** — bytes in/out

Metrics are collected every 30 seconds by a background worker.

### Real-time Logs

Logs are streamed via Server-Sent Events (SSE):

- **Runtime logs** — live output from your container (stdout/stderr)
- **Build logs** — full output from the Docker build process

Logs are displayed newest-first with pagination support.

### Alerts

Configure threshold-based alerts:

1. Go to **App Detail → Alerts**
2. Click **Add Alert**
3. Set a metric (CPU or memory), threshold, and comparison operator
4. Alerts are evaluated every metrics collection cycle

## Subdomains

Every app gets a subdomain: `<name>.luxview.cloud`

- Subdomain uniqueness is enforced at creation time
- SSL certificates are provisioned automatically via Let's Encrypt
- Wildcard DNS (`*.luxview.cloud`) resolves all subdomains to the platform
