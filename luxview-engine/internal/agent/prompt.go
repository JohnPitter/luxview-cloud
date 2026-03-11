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
const dockerfileRules = `
## Dockerfile Generation Rules

### General
- Single container only. Must EXPOSE the port and respond to HTTP GET on / or /health.
- Every line must be a valid Dockerfile instruction or a # comment. No bare words.
- Read the source code to find the actual PORT (e.g. "process.env.PORT ?? 3001" → port 3001). Do NOT guess.
- For CMD: only use "npm start" / "pnpm start" if a "start" script exists in package.json. Otherwise run the entrypoint directly: CMD ["node", "dist/index.js"]. Check the "dev" script to infer the entrypoint (e.g. "tsx watch src/index.ts" → dist/index.js after build).
- Use multi-stage builds for simple projects. Use alpine base images.
- Delete .test.ts / .spec.ts files before build: RUN find . -name "*.test.ts" -o -name "*.test.tsx" -o -name "*.spec.ts" | xargs rm -f

### pnpm Monorepos (pnpm-workspace.yaml present)
DO NOT use multi-stage builds — pnpm symlinks break with COPY --from=builder.
Use a single stage with this pattern:

  FROM node:20-alpine
  ENV CI=true
  RUN corepack enable && corepack prepare pnpm@latest --activate
  WORKDIR /app
  COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json* ./
  COPY packages/ ./packages/
  RUN rm -rf packages/*/node_modules && pnpm install --frozen-lockfile
  # If shared package has "main": "./src/index.ts", patch it:
  # RUN sed -i 's|"./src/index.ts"|"./dist/index.js"|g' packages/<shared>/package.json
  # If Prisma is used, generate client before build:
  # RUN cd packages/<api> && npx prisma generate
  RUN pnpm build
  # Save Prisma client if applicable:
  # RUN cp -r node_modules/.pnpm/@prisma+client*/node_modules/.prisma /tmp/.prisma
  RUN rm -rf node_modules packages/*/node_modules
  RUN pnpm install --frozen-lockfile --prod
  # Restore Prisma client if applicable:
  # RUN find node_modules/.pnpm -path '*/@prisma/client' -type d | head -1 | xargs -I{} cp -r /tmp/.prisma {}/../../.prisma
  EXPOSE <port>
  CMD ["node", "packages/<api>/dist/index.js"]

Key principles:
- WORKDIR is always the monorepo ROOT (/app), never a subdirectory.
- CMD runs from the root. NEVER use "cd" in CMD/ENTRYPOINT.
- After COPY of source dirs, always rm -rf node_modules and reinstall (host symlinks don't work in container).
- Keep ALL build output (packages/*/dist/) — the API may serve the frontend's dist/ via express.static() with relative paths from __dirname.

### Default Ports (only if source code doesn't specify)
Node.js/Next.js: 3000 | Python: 8000 | Go/Java/Rust: 8080 | Vite SPA/Static (nginx): 80
`

// Managed services reference shared by both prompts.
const servicesReference = `
## LuxView Cloud Managed Services
- PostgreSQL: DATABASE_URL, PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
- Redis: REDIS_URL, REDIS_HOST, REDIS_PORT, REDIS_PASSWORD
- MongoDB: MONGODB_URL, MONGO_URL
- RabbitMQ: RABBITMQ_URL, AMQP_URL
- S3/MinIO: S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY

Detect and recommend replacements:
- SQLite/MySQL/MariaDB/SQL Server/CockroachDB → "postgres"
- Memcached/local cache → "redis"
- Self-hosted Redis/MongoDB/RabbitMQ/PostgreSQL → managed version
- Local file uploads → "s3"
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
