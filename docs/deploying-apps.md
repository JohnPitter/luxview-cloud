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

## Supported Stacks & Requirements

LuxView auto-detects your stack based on files in the root of your repository. If a `Dockerfile` is present, it always takes priority. Below are the requirements for each stack so your deploy succeeds on the first try.

### Dockerfile (Custom)

**Detected by:** `Dockerfile` in root
**Priority:** Highest — overrides all other detection

Use your own Dockerfile for full control. LuxView reads the `EXPOSE` directive to determine which port to route traffic to.

```dockerfile
FROM node:20-alpine
WORKDIR /app
COPY . .
RUN npm ci && npm run build
EXPOSE 3000
CMD ["node", "dist/server.js"]
```

**Requirements:**
- Must have an `EXPOSE` directive (or set the port in the dashboard)
- The container must listen on the exposed port
- The container must respond to HTTP GET on `/` or any path for the health check

---

### Next.js

**Detected by:** `package.json` + (`next.config.js` | `next.config.mjs` | `next.config.ts`)
**Runtime:** Node 20 Alpine
**Default port:** 3000

**Requirements:**
- `package.json` with a `build` script (`next build`)
- `next.config.js` must enable standalone output:
  ```js
  module.exports = { output: 'standalone' }
  ```
- `package-lock.json` is recommended (enables faster `npm ci`) but not required

**What LuxView does:**
1. `npm ci` (or `npm install` if no lockfile)
2. `npm run build`
3. Copies `.next/standalone`, `.next/static`, and `public` to the runtime image
4. Runs `node server.js` on port 3000

---

### Vite (React, Vue, Svelte)

**Detected by:** `package.json` + (`vite.config.js` | `vite.config.ts` | `vite.config.mjs`)
**Runtime:** Nginx Alpine (static serving)
**Default port:** 80

**Requirements:**
- `package.json` with Vite as a dependency
- A Vite config file in the root
- Build output goes to `dist/` (Vite default)

**What LuxView does:**
1. `npm ci` (or `npm install` if no lockfile)
2. `npx vite build --base=/`
3. Serves `dist/` with Nginx, with SPA fallback (`try_files $uri /index.html`)

---

### Node.js (Generic)

**Detected by:** `package.json` (without Next.js or Vite markers)
**Runtime:** Node 20 Alpine
**Default port:** 3000

**Requirements:**
- `package.json` **must have a `"start"` script**. This is the most common cause of deploy failures:
  ```json
  {
    "scripts": {
      "start": "node server.js"
    }
  }
  ```
- Your app must listen on the port defined by `PORT` env var or 3000
- `package-lock.json` is recommended but not required

**What LuxView does:**
1. `npm ci --omit=dev` (or `npm install --omit=dev` if no lockfile)
2. `npm start`

> **Common error:** `Missing script: "start"` — add a `start` script to your `package.json`.

---

### Python

**Detected by:** `requirements.txt` | `pyproject.toml` | `Pipfile`
**Runtime:** Python 3.12 Slim
**Default port:** 8000

**Requirements:**
- At least one of: `requirements.txt`, `pyproject.toml`, or `Pipfile`
- **Gunicorn** must be in your dependencies — the default CMD runs:
  ```
  gunicorn --bind 0.0.0.0:8000 --workers 2 app:app
  ```
- Your WSGI app must be accessible as `app:app` (a variable named `app` in a file named `app.py`). If your entry point is different, use a custom `Dockerfile`

**What LuxView does:**
1. Installs dependencies from `requirements.txt`, `pyproject.toml`, or `Pipfile`
2. Runs Gunicorn pointing to `app:app` on port 8000

> **Tip:** For Flask, ensure your file is named `app.py` with `app = Flask(__name__)`. For Django, use a custom Dockerfile with `gunicorn myproject.wsgi:application`.

---

### Go

**Detected by:** `go.mod`
**Runtime:** Go 1.22 (build) → Alpine 3.19 (run)
**Default port:** 8080

**Requirements:**
- `go.mod` in the root
- The project must compile with `go build ./...`
- The resulting binary must listen on port 8080 (or the port set in the dashboard)

**What LuxView does:**
1. `go mod download`
2. `CGO_ENABLED=0 GOOS=linux go build -o /server ./...`
3. Runs `./server` on Alpine with ca-certificates

> **Tip:** If you have multiple `main` packages, use a custom Dockerfile to specify the build target: `go build -o /server ./cmd/myapp`

---

### Java (Spring Boot)

**Detected by:** `pom.xml` | `build.gradle` | `build.gradle.kts`
**Runtime:** Eclipse Temurin JDK (build) → JRE (run)
**Default port:** 8080

LuxView auto-detects the Java version from your build files (supports 8, 11, 17, 21; defaults to 21).

**Requirements:**
- Maven or Gradle project with a Spring Boot application
- For Maven: `mvn clean package` must produce a JAR in `target/`
- For Gradle: `gradle clean bootJar` must produce a JAR in `build/libs/`
- The app uses `SERVER_PORT` env var (auto-set to 8080)

**What LuxView does:**
1. Builds with Maven (`mvn clean package -DskipTests`) or Gradle (`gradle clean bootJar -x test`)
2. Uses `mvnw`/`gradlew` wrapper if present
3. Runs `java -jar app.jar`

> **Tip:** Include `mvnw` or `gradlew` in your repo to ensure consistent build tool versions.

---

### Rust

**Detected by:** `Cargo.toml`
**Runtime:** Rust 1.77 Slim (build) → Debian Bookworm Slim (run)
**Default port:** 8080

**Requirements:**
- `Cargo.toml` in the root
- A binary crate (the compiled binary must be in `target/release/`)
- The binary must listen on port 8080

**What LuxView does:**
1. Caches dependency build (dummy `main.rs` trick)
2. `cargo build --release`
3. Copies the release binary to a slim Debian image

> **Note:** Build times for Rust can be long (3-10 minutes). If the health check times out, consider using a custom Dockerfile with a pre-built binary or multi-stage caching.

---

### Static Site

**Detected by:** `index.html` in root (no other stack markers)
**Runtime:** Nginx Alpine
**Default port:** 80

**Requirements:**
- An `index.html` file in the root directory
- All assets (CSS, JS, images) should be referenced with relative paths

**What LuxView does:**
1. Copies all files to Nginx's html directory
2. Serves with SPA fallback (`try_files $uri /index.html`)

---

### Detection Priority

If multiple markers are present, LuxView uses this priority order:

1. **Dockerfile** (custom — always wins)
2. **Next.js** (`package.json` + `next.config.*`)
3. **Vite** (`package.json` + `vite.config.*`)
4. **Node.js** (`package.json`)
5. **Python** (`requirements.txt` / `pyproject.toml` / `Pipfile`)
6. **Go** (`go.mod`)
7. **Java** (`pom.xml` / `build.gradle`)
8. **Rust** (`Cargo.toml`)
9. **Static** (`index.html`)

> **Tip:** If LuxView detects the wrong stack, add a `Dockerfile` to your repo for full control.

---

### Health Checks

After starting your container, LuxView performs HTTP health checks:

- **Method:** HTTP GET on the container's port
- **Timeout:** 120s (180s for Java, Next.js, and custom Dockerfile)
- **Interval:** Every 2 seconds
- **Expected:** Any HTTP 2xx response

If the health check fails, the deploy is rolled back and the error appears in the build logs.

**Common health check failures:**
- App not listening on the expected port → set the correct port in the dashboard
- App crashes on startup → check the build logs for error messages
- Missing `start` script (Node.js) → add `"start"` to `package.json` scripts
- Missing `gunicorn` (Python) → add it to `requirements.txt`
- Slow startup (Java/Rust) → consider increasing timeout with a custom Dockerfile

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
