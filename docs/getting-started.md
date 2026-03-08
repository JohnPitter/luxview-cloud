# Getting Started

Welcome to LuxView Cloud — a self-hosted PaaS that lets you deploy applications directly from GitHub.

## How it works

1. **Sign in** with your GitHub account
2. **Select** a repository and branch
3. **Configure** subdomain, environment variables, and resource limits
4. **Deploy** — LuxView detects your stack, builds a Docker image, starts a container, and provisions HTTPS

Your app is live at `https://<subdomain>.luxview.cloud` within seconds.

## Supported Stacks

LuxView auto-detects your project stack based on files in the repository:

| File Detected | Stack | Runtime | Key Requirement |
|---|---|---|---|
| `Dockerfile` | Custom | Your Dockerfile | Must have `EXPOSE` directive |
| `package.json` + `next.config.*` | Next.js | Node 20 | `output: 'standalone'` in next.config |
| `package.json` + `vite.config.*` | Vite | Nginx (static) | Build output in `dist/` |
| `package.json` | Node.js | Node 20 | **Must have a `"start"` script** |
| `requirements.txt` / `Pipfile` | Python | Python 3.12 | `gunicorn` in dependencies, entry point `app:app` |
| `go.mod` | Go | Go 1.22 | Compiles with `go build ./...` |
| `pom.xml` / `build.gradle` | Java | JDK 21 | Spring Boot with JAR output |
| `Cargo.toml` | Rust | Rust 1.77 | Binary crate in `target/release/` |
| `index.html` (no other markers) | Static | Nginx | All assets with relative paths |

If your project has a `Dockerfile`, it takes priority over all other detection methods.

> For detailed requirements per stack (common errors, tips, and examples), see [Deploying Apps → Supported Stacks](./deploying-apps.md#supported-stacks--requirements).

## Your First Deploy

### 1. Sign in

Go to the LuxView Cloud landing page and click **Sign in with GitHub**. This uses GitHub OAuth — no passwords are stored.

### 2. Create an App

From the dashboard, click **New App**. You'll see a list of your GitHub repositories.

- **Select a repository** — click to choose
- **Pick a branch** — defaults to `main`
- **Set a subdomain** — this becomes `<subdomain>.luxview.cloud`
- **Configure resources** (optional) — CPU limit, memory limit
- **Add environment variables** (optional) — encrypted at rest with AES-256-GCM

### 3. Deploy

Click **Deploy**. LuxView will:

1. Clone your repository
2. Detect the stack (or use your Dockerfile)
3. Build a Docker image
4. Start an isolated container
5. Provision an SSL certificate
6. Run health checks
7. Route traffic to your app

You can watch the build progress in real time on the app detail page.

### 4. Access Your App

Once deployed, your app is available at:

```
https://<subdomain>.luxview.cloud
```

SSL is automatic via Let's Encrypt — no configuration needed.

## Dashboard Overview

After signing in, the dashboard shows:

- **App list** — all your applications with status indicators (Running, Building, Stopped, Error)
- **Quick metrics** — CPU and memory usage per app
- **Recent activity** — latest deployments and status changes

### App Detail Page

Click any app to see:

- **Overview** — status, subdomain URL, stack, branch, resource usage
- **Metrics** — real-time CPU, memory, and network charts
- **Logs** — live-streamed runtime logs + build logs for each deployment
- **Services** — attached databases, caches, and storage
- **Deployments** — full deployment history with rollback capability

## What's Next

- [Deploying Apps](./deploying-apps.md) — auto-deploy, rollback, environment variables
- [Services](./services.md) — databases, Redis, MongoDB, RabbitMQ, S3 storage
- [Admin Guide](./admin-guide.md) — user management, plans, platform settings
- [API Reference](./api-reference.md) — REST API endpoints
- [Self-Hosting](./self-hosting.md) — deploy LuxView on your own VPS
