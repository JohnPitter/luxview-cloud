package agent

import (
	"fmt"
	"os"
	"path/filepath"
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

Notes: for Gradle replace Maven with: COPY build.gradle* settings.gradle* gradlew* ./ && COPY gradle ./gradle && RUN ./gradlew build -x test. Detect java.version in pom.xml to pick JDK version (8/11/17/21).

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

  FROM node:20-alpine
  ENV CI=true NODE_ENV=production
  RUN corepack enable && corepack prepare pnpm@latest --activate
  WORKDIR /app
  COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json* ./
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
  EXPOSE 3001
  CMD ["node", "packages/api/dist/index.js"]

Adapt: only include Prisma lines if @prisma/client is in dependencies. Only include shared "main" patch if needed. Replace package names with actual ones. WORKDIR is always /app (root). CMD runs from root — NEVER "cd" in CMD. Keep ALL dist/ output (API may serve frontend static files).
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
- Local file uploads / S3 usage → "storage"
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

	sb.WriteString("Diagnose the build failure and provide a corrected Dockerfile.")

	return sb.String(), nil
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
