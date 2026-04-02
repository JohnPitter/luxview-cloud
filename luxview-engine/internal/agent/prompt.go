package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/luxview/engine/pkg/logger"
)

const maxFilesInTree = 200
const maxFileSize = 16 * 1024    // 16KB per file
const maxTotalContext = 50 * 1024 // 50KB total context for deploy analysis

// Directories to skip when scanning the repo.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"target":       true,
	"__pycache__":  true,
}

// Key files to read for context.
var keyFiles = []string{
	"package.json",
	"go.mod",
	"requirements.txt",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	"next.config.js",
	"next.config.mjs",
	"next.config.ts",
	"vite.config.js",
	"vite.config.ts",
	"vite.config.mjs",
	"Cargo.toml",
	"pom.xml",
	"build.gradle",
	"index.html",
	"pnpm-workspace.yaml",
	"turbo.json",
	"Makefile",
	"main.go",
	"app.py",
	"manage.py",
	"prisma/schema.prisma",
	"drizzle.config.ts",
	"drizzle.config.js",
	"knexfile.js",
	"knexfile.ts",
	"ormconfig.json",
	"ormconfig.ts",
	"config/database.yml",
	"alembic.ini",
	".env.example",
}

// Monorepo glob patterns for additional package.json files.
var monorepoPatterns = []string{
	"apps/*/package.json",
	"packages/*/package.json",
	"apps/*/Dockerfile",
	"packages/*/Dockerfile",
}

// Shared Dockerfile rules used by both analysis and failure prompts.
// Each template below was tested in Docker with a real health check.
const dockerfileRules = `
## Rules
- Single container. Must EXPOSE the port and respond to HTTP GET on / or /health.
- Every line must be a valid Dockerfile instruction or a # comment.
- Read the source code to find the actual PORT. Do NOT guess.
- For Node.js CMD: only use "npm start" if a "start" script exists. Otherwise run the entrypoint directly.
- Pick the template that matches the detected stack and adapt it.
- IMPORTANT: Detect system dependencies used by the app:
  1. If the code calls execFile/spawn/exec with system binaries (git, grep, rg, curl, ffmpeg, etc.), add "RUN apk add --no-cache <package>" for Alpine. Common: git→git, rg→ripgrep, ffmpeg→ffmpeg.
  2. If package.json dependencies include packages that need a browser (puppeteer, @puppeteer, playwright, @wppconnect-team/wppconnect, whatsapp-web.js, chrome-launcher), add Chromium: "RUN apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont" and set ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser and ENV PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true.
  3. If package.json has sharp or canvas, add build deps: "RUN apk add --no-cache vips-dev build-base" for sharp, "RUN apk add --no-cache cairo-dev pango-dev jpeg-dev giflib-dev" for canvas.

## Templates (all Docker-tested)

### Node.js (Express/Fastify/Koa)
  FROM node:20-alpine AS builder
  WORKDIR /app
  COPY package.json package-lock.json* ./
  RUN npm ci
  COPY . .
  RUN npm run build
  FROM node:20-alpine
  WORKDIR /app
  COPY package.json package-lock.json* ./
  RUN npm ci --omit=dev
  COPY --from=builder /app/dist ./dist
  ENV NODE_ENV=production
  EXPOSE 3000
  CMD ["node", "dist/index.js"]

Notes: read "dev" script to infer entrypoint (e.g. "tsx watch src/index.ts" → dist/index.js). Read source for actual PORT.

### Next.js
  FROM node:20-alpine AS builder
  WORKDIR /app
  COPY package.json package-lock.json* ./
  RUN npm ci
  COPY . .
  RUN npm run build
  FROM node:20-alpine
  WORKDIR /app
  COPY --from=builder /app/.next/standalone ./
  COPY --from=builder /app/.next/static ./.next/static
  COPY --from=builder /app/public ./public
  ENV NODE_ENV=production
  EXPOSE 3000
  CMD ["node", "server.js"]

Notes: requires "output": "standalone" in next.config. If not present, add a suggestion to the user.

### Vite SPA (React/Vue/Svelte)
  FROM node:20-alpine AS builder
  WORKDIR /app
  COPY package.json package-lock.json* ./
  RUN npm ci
  COPY . .
  RUN npm run build
  FROM nginx:alpine
  COPY --from=builder /app/dist /usr/share/nginx/html
  RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  index index.html;\n  location / {\n    try_files $uri $uri/ /index.html;\n  }\n}\n' > /etc/nginx/conf.d/default.conf
  EXPOSE 80
  CMD ["nginx", "-g", "daemon off;"]

### Python (FastAPI/Flask/Django)
  FROM python:3.12-slim
  WORKDIR /app
  COPY requirements.txt* pyproject.toml* ./
  RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || pip install --no-cache-dir . 2>/dev/null || true
  COPY . .
  ENV PYTHONUNBUFFERED=1
  EXPOSE 8000
  CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]

Notes: for Django use CMD ["gunicorn", "project.wsgi:application", "--bind", "0.0.0.0:8000"]. For Flask use CMD ["gunicorn", "app:app", "--bind", "0.0.0.0:8000"]. Read source for the actual module name and app variable.

### Go
  FROM golang:alpine AS builder
  WORKDIR /app
  RUN apk add --no-cache git
  COPY go.mod go.sum* ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 GOOS=linux go build -o /server .
  FROM alpine:3.20
  RUN apk --no-cache add ca-certificates
  COPY --from=builder /server /server
  EXPOSE 8080
  CMD ["/server"]

Notes: if main.go is in cmd/<name>/, adjust build path: go build -o /server ./cmd/<name>

### Java (Maven)
  FROM maven:3.9-eclipse-temurin-21 AS builder
  WORKDIR /app
  COPY pom.xml ./
  RUN mvn dependency:go-offline -q
  COPY src ./src
  RUN mvn package -DskipTests -q
  FROM eclipse-temurin:21-jre-alpine
  WORKDIR /app
  COPY --from=builder /app/target/*.jar app.jar
  EXPOSE 8080
  CMD ["java", "-jar", "app.jar"]

Notes:
- Detect java.version in pom.xml to pick JDK version (8/11/17/21).
- For multi-module Maven projects: use -pl :module-name -am to build only the runnable module and its dependencies. COPY the entire project, not just src/.
- CRITICAL: If a module is defined inside a <profile> in pom.xml (not in the default <modules>), you MUST activate that profile with -P<profileName>. Check pom.xml for <profiles> that contain <module> entries matching the target module.
- For Gradle replace Maven with: COPY build.gradle* settings.gradle* gradlew* ./ && COPY gradle ./gradle && RUN ./gradlew build -x test.

### Java (Gradle)
  FROM gradle:8-jdk21 AS builder
  WORKDIR /app
  COPY build.gradle* settings.gradle* gradlew* ./
  COPY gradle ./gradle
  COPY src ./src
  RUN gradle build -x test --no-daemon
  FROM eclipse-temurin:21-jre-alpine
  WORKDIR /app
  COPY --from=builder /app/build/libs/*.jar app.jar
  EXPOSE 8080
  CMD ["java", "-jar", "app.jar"]

### Rust
  FROM rust:alpine AS builder
  RUN apk add --no-cache musl-dev
  WORKDIR /app
  COPY Cargo.toml Cargo.lock* ./
  RUN mkdir src && echo "fn main(){}" > src/main.rs && cargo build --release && rm -rf src
  COPY src ./src
  RUN cargo build --release
  FROM alpine:3.20
  RUN apk --no-cache add ca-certificates
  COPY --from=builder /app/target/release/<binary_name> /server
  EXPOSE 8080
  CMD ["/server"]

Notes: replace <binary_name> with the name from Cargo.toml [package].name.

### Static (HTML/CSS/JS)
  FROM nginx:alpine
  COPY . /usr/share/nginx/html
  RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  index index.html;\n  location / {\n    try_files $uri $uri/ /index.html;\n  }\n}\n' > /etc/nginx/conf.d/default.conf
  EXPOSE 80
  CMD ["nginx", "-g", "daemon off;"]

### pnpm Monorepo (pnpm-workspace.yaml present)
DO NOT use multi-stage builds — pnpm symlinks break with COPY --from=builder.
CRITICAL: NODE_ENV must NOT be "production" during install/build — devDependencies (tsup, typescript, etc.) are needed. Set NODE_ENV=production only AFTER the build.

  FROM node:20-alpine
  ENV CI=true
  RUN corepack enable && corepack prepare pnpm@latest --activate
  WORKDIR /app
  COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json* tsconfig.json* ./
  COPY packages/ ./packages/
  RUN rm -rf packages/*/node_modules && pnpm install --frozen-lockfile
  # If shared package "main" points to .ts, patch to .js:
  RUN sed -i 's|"./src/index.ts"|"./dist/index.js"|g' packages/shared/package.json
  # If Prisma: generate before build, save version for re-generation
  RUN cd packages/api && npx prisma generate
  RUN cd packages/api && node -e "console.log(require('@prisma/client/package.json').version)" > /tmp/prisma-version
  RUN find . -name "*.test.ts" -o -name "*.test.tsx" -o -name "*.spec.ts" | xargs rm -f 2>/dev/null || true
  RUN pnpm build
  RUN rm -rf node_modules packages/*/node_modules
  RUN pnpm install --frozen-lockfile --prod
  # Re-generate Prisma client (prod install loses it). Pin version to avoid incompatibility.
  RUN cd packages/api && npx --yes prisma@$(cat /tmp/prisma-version) generate
  ENV NODE_ENV=production
  EXPOSE 3001
  CMD ["node", "packages/api/dist/index.js"]

Adapt:
- CRITICAL: Only include Prisma lines (prisma generate, prisma-version, etc.) if @prisma/client appears in a package.json dependencies. If the project uses Drizzle, Knex, TypeORM, or any other ORM — do NOT add any Prisma lines.
- Only include shared "main" patch if needed.
- Replace package names with actual ones.
- WORKDIR is always /app (root). CMD runs from root — NEVER "cd" in CMD.
- Keep ALL dist/ output (API may serve frontend static files).
- Always include tsconfig.json in COPY if it exists (packages extend it via "extends": "../../tsconfig.json").
- Also include apps/ directory if the app code is there, not just packages/.
`

// Managed services reference shared by both prompts.
const servicesReference = `
## LuxView Cloud Managed Services
- PostgreSQL: DATABASE_URL, PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
- Redis: REDIS_URL, REDIS_HOST, REDIS_PORT, REDIS_PASSWORD
- MongoDB: MONGODB_URL, MONGO_URL
- RabbitMQ: RABBITMQ_URL, AMQP_URL
- Storage: STORAGE_PATH (local disk volume mounted at /storage)

Detect and recommend replacements:
- SQLite/MySQL/MariaDB/SQL Server/CockroachDB → "postgres"
- Memcached/local cache → "redis"
- Self-hosted Redis/MongoDB/RabbitMQ/PostgreSQL → managed version
- Local file uploads → "storage"
- multer / formidable / busboy / express-fileupload / sharp (image processing) → "storage"
- fs.writeFile / fs.createWriteStream with uploads directory → "storage"
- Any file upload middleware or local file persistence → "storage"

IMPORTANT: When recommending "storage", also add STORAGE_PATH=/storage as an ENV in the Dockerfile.
The platform will auto-provision the storage volume on deploy when it detects STORAGE_PATH in the Dockerfile.
`

const systemPrompt = `You are a Deploy Agent for LuxView Cloud, a self-hosted PaaS platform.
Analyze the repository and generate an optimal Dockerfile for deployment. Also detect external services and recommend managed alternatives.
` + dockerfileRules + servicesReference + `
## Service Recommendations
For each detected service: set "currentEvidence" to the file where you found it, provide 3-6 "manualSteps", and omit "codeChanges".

## Environment Variables Detection
You MUST include ALL environment variables found in the "Environment Variables Found in Source Code" section in the envHints array. Do not skip any.
Rules:
- Mark as required=true if the code throws/exits when the variable is missing
- Mark as required=false if there's a default fallback value — but STILL include it (the default is usually for development only and won't work in production)
- For URL variables (callback URLs, frontend URLs, webhook URLs), always include them — localhost defaults need to be changed for production
- For secret/key variables, note in the description that the user should generate a secure value
- Do NOT include these platform-injected variables: DATABASE_URL, PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE, REDIS_URL, MONGODB_URL, RABBITMQ_URL, STORAGE_PATH, SPRING_DATASOURCE_*
- Do NOT include these Dockerfile-set variables: NODE_ENV, PORT, CI, ORCHESTRATOR_PORT (or similar port vars already set via ENV in the Dockerfile)

## Response Format
Respond with valid JSON only — no markdown, no extra text.
{
  "suggestions": [{"type": "error|warning|info", "message": "..."}],
  "dockerfile": "FROM ...\n...",
  "port": 3000,
  "stack": "nodejs|nextjs|vite|python|go|java|rust|static",
  "envHints": [{"key": "DATABASE_URL", "description": "...", "required": true}],
  "serviceRecommendations": [{"currentService": "sqlite", "currentEvidence": "package.json: better-sqlite3", "recommendedService": "postgres", "reason": "...", "manualSteps": ["..."]}]
}`

const failureSystemPrompt = `You are a Deploy Agent for LuxView Cloud, a self-hosted PaaS platform.
A deployment has FAILED. Analyze the repository, the Dockerfile used, and the build log. Diagnose the exact cause and provide a corrected Dockerfile.
` + dockerfileRules + `
## Environment Variables Detection
You MUST include ALL environment variables found in the "Environment Variables Found in Source Code" section in the envHints array. Do not skip any.
Rules:
- Mark as required=true if the code throws/exits when the variable is missing
- Mark as required=false if there's a default fallback value — but STILL include it (the default is usually for development only and won't work in production)
- For URL variables (callback URLs, frontend URLs, webhook URLs), always include them — localhost defaults need to be changed for production
- For secret/key variables, note in the description that the user should generate a secure value
- Do NOT include these platform-injected variables: DATABASE_URL, PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE, REDIS_URL, MONGODB_URL, RABBITMQ_URL, STORAGE_PATH, SPRING_DATASOURCE_*
- Do NOT include these Dockerfile-set variables: NODE_ENV, PORT, CI, ORCHESTRATOR_PORT (or similar port vars already set via ENV in the Dockerfile)

## Response Format
Respond with valid JSON only — no markdown, no extra text.
{
  "suggestions": [{"type": "error|warning|info", "message": "..."}],
  "dockerfile": "FROM ...\n...",
  "port": 3000,
  "stack": "nodejs|nextjs|vite|python|go|java|rust|static",
  "envHints": [{"key": "DATABASE_URL", "description": "...", "required": true}],
  "diagnosis": "Root cause explanation..."
}`

// BuildContext scans the repository and builds a user prompt for first-deploy analysis.
func BuildContext(repoDir string) (string, error) {
	log := logger.With("deploy-agent")

	tree, err := buildFileTree(repoDir)
	if err != nil {
		return "", fmt.Errorf("build file tree: %w", err)
	}

	files, err := readKeyFiles(repoDir)
	if err != nil {
		log.Warn().Err(err).Msg("partial error reading key files")
	}

	// Check for monorepo indicators
	_, hasPnpmWorkspace := files["pnpm-workspace.yaml"]
	_, hasTurboJson := files["turbo.json"]
	isMonorepo := hasPnpmWorkspace || hasTurboJson
	log.Debug().Int("key_files_found", len(files)).Bool("is_monorepo", isMonorepo).Msg("key files read for context")

	var sb strings.Builder
	sb.WriteString("## Repository File Tree\n```\n")
	sb.WriteString(tree)
	sb.WriteString("```\n\n")

	if len(files) > 0 {
		sb.WriteString("## Key File Contents\n\n")
		for name, content := range files {
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", name, content))
		}
	}

	// Scan source files for environment variable references and system binary usage
	envVars, sysBins := scanSourceCode(repoDir)
	if len(envVars) > 0 {
		sb.WriteString("## Environment Variables Found in Source Code\n")
		sb.WriteString("The following environment variables were detected by scanning source files:\n```\n")
		for _, ev := range envVars {
			sb.WriteString(fmt.Sprintf("%s (found in %s)\n", ev.name, ev.file))
		}
		sb.WriteString("```\n\n")
		log.Debug().Int("env_vars_found", len(envVars)).Msg("environment variables scanned from source")
	}
	if len(sysBins) > 0 {
		sb.WriteString("## System Binaries Used in Source Code\n")
		sb.WriteString("The following system binaries are called via execFile/spawn/exec:\n```\n")
		for _, bin := range sysBins {
			sb.WriteString(fmt.Sprintf("%s (found in %s)\n", bin.name, bin.file))
		}
		sb.WriteString("```\nIMPORTANT: These must be installed in the Dockerfile (e.g. RUN apk add --no-cache git).\n\n")
		log.Debug().Int("sys_bins_found", len(sysBins)).Msg("system binaries scanned from source")
	}

	// Detect npm deps that require system-level packages
	nativeDeps := scanNativeDeps(repoDir)
	if len(nativeDeps) > 0 {
		sb.WriteString("## Dependencies Requiring System Packages\n")
		sb.WriteString("These npm/pip packages require system-level binaries or libraries:\n```\n")
		for _, d := range nativeDeps {
			sb.WriteString(fmt.Sprintf("%s (in %s) → requires: %s\n", d.pkg, d.file, d.requires))
		}
		sb.WriteString("```\nIMPORTANT: Install these system packages in the Dockerfile BEFORE npm/pnpm install.\n\n")
		log.Debug().Int("native_deps", len(nativeDeps)).Msg("native dependencies detected")
	}

	sb.WriteString("Analyze this repository and generate an optimal Dockerfile for deployment on LuxView Cloud.")
	totalContext := sb.Len()
	log.Debug().Int("total_context_size", totalContext).Msg("context built for LLM")

	return sb.String(), nil
}

// BuildFailureContext builds a user prompt for failure diagnosis, including build log and Dockerfile.
func BuildFailureContext(repoDir, buildLog, dockerfile string) (string, error) {
	log := logger.With("deploy-agent")

	tree, err := buildFileTree(repoDir)
	if err != nil {
		return "", fmt.Errorf("build file tree: %w", err)
	}

	files, err := readKeyFiles(repoDir)
	if err != nil {
		log.Warn().Err(err).Msg("partial error reading key files")
	}

	// Truncate build log to last 4KB
	const maxBuildLog = 4 * 1024
	if len(buildLog) > maxBuildLog {
		buildLog = buildLog[len(buildLog)-maxBuildLog:]
	}

	var sb strings.Builder
	sb.WriteString("## Repository File Tree\n```\n")
	sb.WriteString(tree)
	sb.WriteString("```\n\n")

	if len(files) > 0 {
		sb.WriteString("## Key File Contents\n\n")
		for name, content := range files {
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", name, content))
		}
	}

	sb.WriteString("## Dockerfile Used\n```dockerfile\n")
	sb.WriteString(dockerfile)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Build Log (last 4KB)\n```\n")
	sb.WriteString(buildLog)
	sb.WriteString("\n```\n\n")

	// Scan source files for environment variable references and system binary usage
	envVars, sysBins := scanSourceCode(repoDir)
	if len(envVars) > 0 {
		sb.WriteString("## Environment Variables Found in Source Code\n```\n")
		for _, ev := range envVars {
			sb.WriteString(fmt.Sprintf("%s (found in %s)\n", ev.name, ev.file))
		}
		sb.WriteString("```\n\n")
	}
	if len(sysBins) > 0 {
		sb.WriteString("## System Binaries Used in Source Code\n```\n")
		for _, bin := range sysBins {
			sb.WriteString(fmt.Sprintf("%s (found in %s)\n", bin.name, bin.file))
		}
		sb.WriteString("```\nIMPORTANT: These must be installed in the Dockerfile.\n\n")
	}

	nativeDeps := scanNativeDeps(repoDir)
	if len(nativeDeps) > 0 {
		sb.WriteString("## Dependencies Requiring System Packages\n```\n")
		for _, d := range nativeDeps {
			sb.WriteString(fmt.Sprintf("%s (in %s) → requires: %s\n", d.pkg, d.file, d.requires))
		}
		sb.WriteString("```\n\n")
	}

	sb.WriteString("Diagnose the build failure and provide a corrected Dockerfile.")

	return sb.String(), nil
}

// sourceRef represents a reference found in source code (env var or system binary).
type sourceRef struct {
	name string
	file string
}

// nativeDep represents an npm/pip package that requires system-level binaries.
type nativeDep struct {
	pkg      string
	file     string
	requires string
}

// Packages that require system-level binaries or libraries.
var nativeDepMap = map[string]string{
	"puppeteer":                     "chromium nss freetype harfbuzz ca-certificates ttf-freefont (+ ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true)",
	"puppeteer-core":                "chromium nss freetype harfbuzz ca-certificates ttf-freefont (+ ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser)",
	"puppeteer-extra":               "chromium nss freetype harfbuzz ca-certificates ttf-freefont (+ ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true)",
	"@wppconnect-team/wppconnect":   "chromium nss freetype harfbuzz ca-certificates ttf-freefont (+ ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true)",
	"whatsapp-web.js":               "chromium nss freetype harfbuzz ca-certificates ttf-freefont (+ ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true)",
	"playwright":                    "chromium nss freetype harfbuzz ca-certificates ttf-freefont",
	"@playwright/test":              "chromium nss freetype harfbuzz ca-certificates ttf-freefont",
	"chrome-launcher":               "chromium nss freetype harfbuzz ca-certificates ttf-freefont",
	"sharp":                         "vips-dev (already handled by npm, but may need: apk add --no-cache vips-dev)",
	"canvas":                        "cairo-dev pango-dev jpeg-dev giflib-dev build-base",
	"bcrypt":                        "build-base python3",
	"better-sqlite3":                "build-base python3",
	"node-gyp":                      "build-base python3 make g++",
	"@mapbox/node-pre-gyp":          "build-base python3",
	"grpc":                          "build-base",
	"@grpc/grpc-js":                 "(no native deps)",
	"sqlite3":                       "build-base python3 sqlite-dev",
	"pg-native":                     "postgresql-dev build-base",
	"ssh2":                          "build-base",
}

// scanNativeDeps scans all package.json files for dependencies that require system packages.
func scanNativeDeps(repoDir string) []nativeDep {
	var results []nativeDep
	seen := make(map[string]bool)

	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.Name() != "package.json" {
			return nil
		}

		content, readErr := readFileLimited(path)
		if readErr != nil || len(content) == 0 {
			return nil
		}

		rel, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		for depName, requires := range nativeDepMap {
			if strings.Contains(requires, "no native") {
				continue
			}
			// Check if dep appears in dependencies or devDependencies
			if strings.Contains(content, `"`+depName+`"`) && !seen[depName] {
				seen[depName] = true
				results = append(results, nativeDep{pkg: depName, file: rel, requires: requires})
			}
		}
		return nil
	})

	sort.Slice(results, func(i, j int) bool { return results[i].pkg < results[j].pkg })
	return results
}

// scanSourceCode scans source files for environment variable references and system binary calls.
// Returns deduplicated lists of env vars and system binaries with the files where they were found.
func scanSourceCode(repoDir string) (envVars []sourceRef, sysBins []sourceRef) {
	log := logger.With("deploy-agent")
	seenEnv := make(map[string]string)
	seenBin := make(map[string]string)

	// Env var patterns
	envPatterns := []*regexp.Regexp{
		regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]+)`),
		regexp.MustCompile(`os\.Getenv\("([A-Z][A-Z0-9_]+)"\)`),
		regexp.MustCompile(`os\.environ\[["']([A-Z][A-Z0-9_]+)["']\]`),
		regexp.MustCompile(`os\.environ\.get\(["']([A-Z][A-Z0-9_]+)["']`),
		regexp.MustCompile(`env\(["']([A-Z][A-Z0-9_]+)["']`),
	}

	// System binary patterns: execFile("git"...), spawn("git"...), exec.Command("git"...)
	binPatterns := []*regexp.Regexp{
		regexp.MustCompile(`execFile\w*\(\s*["']([a-z][\w-]+)["']`),
		regexp.MustCompile(`spawn\(\s*["']([a-z][\w-]+)["']`),
		regexp.MustCompile(`exec\.Command\w*\(\s*["']([a-z][\w-]+)["']`),
		regexp.MustCompile(`subprocess\.\w+\(\s*\[?\s*["']([a-z][\w-]+)["']`),
	}

	// Known system binaries (not npm packages or node builtins)
	knownBins := map[string]bool{
		"git": true, "grep": true, "rg": true, "curl": true, "wget": true,
		"ffmpeg": true, "convert": true, "chromium": true, "chrome": true,
		"python": true, "python3": true, "pip": true, "pip3": true,
		"ssh": true, "scp": true, "rsync": true, "tar": true, "unzip": true,
		"sed": true, "awk": true, "find": true, "make": true, "gcc": true,
		"g++": true, "cmake": true, "openssl": true, "sqlite3": true,
	}

	sourceExts := map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".go": true, ".py": true, ".rb": true, ".rs": true,
		".java": true, ".kt": true,
	}

	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.IsDir() || info.Size() > int64(maxFileSize) {
			return nil
		}

		ext := filepath.Ext(info.Name())
		if !sourceExts[ext] {
			return nil
		}

		content, readErr := readFileLimited(path)
		if readErr != nil || len(content) == 0 {
			return nil
		}

		rel, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// Scan for env vars
		for _, re := range envPatterns {
			for _, m := range re.FindAllStringSubmatch(content, -1) {
				if len(m) >= 2 {
					if _, exists := seenEnv[m[1]]; !exists {
						seenEnv[m[1]] = rel
					}
				}
			}
		}

		// Scan for system binary calls
		for _, re := range binPatterns {
			for _, m := range re.FindAllStringSubmatch(content, -1) {
				if len(m) >= 2 && knownBins[m[1]] {
					if _, exists := seenBin[m[1]]; !exists {
						seenBin[m[1]] = rel
					}
				}
			}
		}

		return nil
	})

	for name, file := range seenEnv {
		envVars = append(envVars, sourceRef{name: name, file: file})
	}
	for name, file := range seenBin {
		sysBins = append(sysBins, sourceRef{name: name, file: file})
	}

	sort.Slice(envVars, func(i, j int) bool { return envVars[i].name < envVars[j].name })
	sort.Slice(sysBins, func(i, j int) bool { return sysBins[i].name < sysBins[j].name })

	log.Debug().Int("env_vars", len(envVars)).Int("sys_bins", len(sysBins)).Msg("source code scan complete")
	return envVars, sysBins
}

// buildFileTree walks the repo and returns a text tree of files (max 200 files).
func buildFileTree(repoDir string) (string, error) {
	log := logger.With("deploy-agent")
	var lines []string
	count := 0

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		if count >= maxFilesInTree {
			return filepath.SkipAll
		}

		rel, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if rel == "." {
			return nil
		}

		if info.IsDir() {
			lines = append(lines, rel+"/")
		} else {
			lines = append(lines, rel)
			count++
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walk repo: %w", err)
	}

	log.Debug().Int("total_files", count).Msg("file tree built")
	return strings.Join(lines, "\n"), nil
}

// readKeyFiles reads key configuration files from the repo, respecting size limits.
func readKeyFiles(repoDir string) (map[string]string, error) {
	log := logger.With("deploy-agent")
	files := make(map[string]string)
	totalSize := 0

	// Read top-level key files
	for _, name := range keyFiles {
		if totalSize >= maxTotalContext {
			break
		}
		content, err := readFileLimited(filepath.Join(repoDir, name))
		if err != nil {
			continue // file doesn't exist or can't be read
		}
		if len(content) == 0 {
			continue
		}
		log.Debug().Str("file", name).Int("size", len(content)).Msg("key file found")
		files[name] = content
		totalSize += len(content)
	}

	// Read monorepo patterns
	for _, pattern := range monorepoPatterns {
		if totalSize >= maxTotalContext {
			break
		}
		fullPattern := filepath.Join(repoDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			log.Debug().Str("pattern", pattern).Err(err).Msg("glob error")
			continue
		}
		for _, match := range matches {
			if totalSize >= maxTotalContext {
				break
			}
			content, err := readFileLimited(match)
			if err != nil || len(content) == 0 {
				continue
			}
			rel, relErr := filepath.Rel(repoDir, match)
			if relErr != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			log.Debug().Str("file", rel).Int("size", len(content)).Msg("monorepo file found")
			files[rel] = content
			totalSize += len(content)
		}
	}

	return files, nil
}

// readFileLimited reads a file up to maxFileSize bytes.
func readFileLimited(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return "", fmt.Errorf("is a directory")
	}

	size := int(info.Size())
	if size == 0 {
		return "", nil
	}

	readSize := min(size, maxFileSize)
	buf := make([]byte, readSize)
	n, err := f.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
