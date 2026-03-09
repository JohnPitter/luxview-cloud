package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxview/engine/pkg/logger"
)

const maxFilesInTree = 200
const maxFileSize = 16 * 1024   // 16KB per file
const maxTotalContext = 50 * 1024  // 50KB total context for deploy analysis
const maxMigrationContext = 256 * 1024 // 256KB total context for migration (needs full codebase)

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

// Source code file extensions to include for full migration context.
var sourceCodeExtensions = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".go": true, ".java": true, ".kt": true, ".rs": true,
	".json": true, ".toml": true, ".yaml": true, ".yml": true,
	".prisma": true, ".graphql": true, ".gql": true, ".sql": true,
	".env.example": true, ".env.sample": true,
}

// Directories to skip when reading source code for migration.
var skipSourceDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	".next": true, "target": true, "__pycache__": true, ".turbo": true,
	"coverage": true, ".cache": true, ".output": true, ".nuxt": true,
	".vercel": true, ".svelte-kit": true, "vendor": true, "venv": true,
	".venv": true, "env": true, ".tox": true, ".mypy_cache": true,
	".pytest_cache": true, ".gradle": true, ".idea": true, ".vscode": true,
}

const systemPrompt = `You are a Deploy Agent for LuxView Cloud, a self-hosted PaaS platform.
Your job is to analyze a user's repository and generate an optimal Dockerfile for deployment.
You also detect external services the app uses and recommend LuxView Cloud managed alternatives.

Supported stacks and their default ports:
- Node.js: port 3000
- Next.js: port 3000
- Vite (React/Vue/Svelte SPA): port 80 (served via nginx)
- Python (Django/Flask/FastAPI): port 8000
- Go: port 8080
- Java (Spring Boot/Maven/Gradle): port 8080
- Rust: port 8080
- Static (HTML/CSS/JS): port 80 (served via nginx)

Dockerfile rules:
1. The app MUST run in a single container.
2. The Dockerfile MUST use EXPOSE to declare the port.
3. The container MUST respond to HTTP GET on / or /health for health checks.
4. Optimize for small images: prefer alpine base images and multi-stage builds.
5. For monorepos, bundle everything into a single container. Identify the main application entry point.
6. Use .dockerignore best practices (node_modules, .git, etc. are already excluded).
7. Install only production dependencies when possible.
8. Set appropriate WORKDIR, COPY, and CMD instructions.

LuxView Cloud managed services (available via platform):
- PostgreSQL: env vars DATABASE_URL, PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
- Redis: env vars REDIS_URL, REDIS_HOST, REDIS_PORT, REDIS_PASSWORD
- MongoDB: env vars MONGODB_URL, MONGO_URL
- RabbitMQ: env vars RABBITMQ_URL, AMQP_URL
- S3/MinIO: env vars S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY

Service detection rules — when you detect these, add a serviceRecommendation:
- SQLite, MySQL, MariaDB, SQL Server, CockroachDB → recommend "postgres"
- Memcached, local file-based cache → recommend "redis"
- Redis (external/self-hosted) → recommend "redis" (managed version)
- MongoDB (external/self-hosted) → recommend "mongodb" (managed version)
- RabbitMQ, ActiveMQ, AMQP → recommend "rabbitmq" (managed version)
- Local file uploads, disk storage → recommend "s3"
- PostgreSQL (external/self-hosted) → recommend "postgres" (managed version)

For each service recommendation:
- Provide 3-6 manual migration steps in "manualSteps"
- Set "currentEvidence" to the file/config where you found the service usage
- Do NOT generate "codeChanges" — leave it empty or omit it (code generation comes later)

You MUST respond with valid JSON only (no markdown, no explanation outside JSON). Use this exact format:
{
  "suggestions": [{"type": "error|warning|info", "message": "..."}],
  "dockerfile": "FROM ...\n...",
  "port": 3000,
  "stack": "nodejs|nextjs|vite|python|go|java|rust|static",
  "envHints": [{"key": "DATABASE_URL", "description": "...", "required": true}],
  "serviceRecommendations": [{"currentService": "sqlite", "currentEvidence": "package.json: better-sqlite3 dependency", "recommendedService": "postgres", "reason": "...", "manualSteps": ["Step 1...", "Step 2..."]}]
}`

const failureSystemPrompt = `You are a Deploy Agent for LuxView Cloud, a self-hosted PaaS platform.
A deployment build has FAILED. Your job is to analyze the repository, the Dockerfile that was used, and the build log to diagnose the failure and provide a corrected Dockerfile.

Supported stacks and their default ports:
- Node.js: port 3000
- Next.js: port 3000
- Vite (React/Vue/Svelte SPA): port 80 (served via nginx)
- Python (Django/Flask/FastAPI): port 8000
- Go: port 8080
- Java (Spring Boot/Maven/Gradle): port 8080
- Rust: port 8080
- Static (HTML/CSS/JS): port 80 (served via nginx)

Rules:
1. The app MUST run in a single container.
2. The Dockerfile MUST use EXPOSE to declare the port.
3. The container MUST respond to HTTP GET on / or /health for health checks.
4. Optimize for small images: prefer alpine base images and multi-stage builds.
5. For monorepos, bundle everything into a single container.
6. Focus on diagnosing the EXACT cause of failure from the build log.
7. Provide a corrected Dockerfile that fixes the issue.

You MUST respond with valid JSON only (no markdown, no explanation outside JSON). Use this exact format:
{
  "suggestions": [{"type": "error|warning|info", "message": "..."}],
  "dockerfile": "FROM ...\n...",
  "port": 3000,
  "stack": "nodejs|nextjs|vite|python|go|java|rust|static",
  "envHints": [{"key": "DATABASE_URL", "description": "...", "required": true}],
  "diagnosis": "Root cause explanation of the build failure..."
}`

const migrationSystemPrompt = `You are a Code Migration Agent for LuxView Cloud, a self-hosted PaaS platform.
Your job is to generate code changes to migrate an application to use a LuxView Cloud managed service.

The service has ALREADY been provisioned. Environment variables are automatically injected at runtime:
- PostgreSQL: DATABASE_URL, PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD
- Redis: REDIS_URL, REDIS_HOST, REDIS_PORT, REDIS_PASSWORD
- MongoDB: MONGODB_URL, MONGO_URL
- RabbitMQ: RABBITMQ_URL, AMQP_URL
- S3/MinIO: S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, AWS_ENDPOINT_URL, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY

## SECURITY RULES (MANDATORY — violations are critical bugs):
1. NEVER hardcode credentials, passwords, connection strings, or secrets in code. Use ONLY environment variables.
2. NEVER add fallback values with real credentials (e.g., "postgres://user:pass@host/db"). Fallbacks must be empty string or throw an error.
3. If a connection string env var is missing, the code MUST throw an error — NEVER silently connect to a default local service.
4. Example of WRONG code: const url = process.env.DATABASE_URL || 'postgresql://postgres:postgres@localhost:5432/mydb'
5. Example of CORRECT code: const url = process.env.DATABASE_URL || ''; if (!url) throw new Error('DATABASE_URL is required');

## COMPLETENESS RULES (MANDATORY — the build MUST pass after your changes):
1. You are given the COMPLETE source code of the repository. You MUST read EVERY file to find ALL imports/requires of the old service module and update ALL of them.
2. If you change a module's exports, check EVERY file in the codebase that imports from that module and update them.
3. If you change package.json dependencies, grep through EVERY source file for imports of the old dependency and update them all.
4. For ORM migrations (e.g., SQLite → PostgreSQL with Drizzle):
   - Update the driver import (e.g., drizzle-orm/libsql → drizzle-orm/postgres-js)
   - Update ALL schema files if they use DB-specific types (e.g., sqliteTable → pgTable)
   - Update ALL files that import the database connection (connection.ts, migrate.ts, seed.ts, etc.)
   - Update drizzle.config.ts/js if it exists
5. Do NOT leave orphan imports — if you remove a dependency, grep for all its usages.
6. Do NOT introduce new exports that don't exist (e.g., don't add "export * from './drizzle'" if there is no drizzle.ts file).
7. Preserve existing exports — if a file exported symbols, the new version must export the same symbols.

## SCOPE RULES (CRITICAL — violations destroy the codebase):
1. ONLY modify files that DIRECTLY use the old database/service being migrated (e.g., files that import the old driver, connection module, or schema types).
2. NEVER modify files that don't import or use the service being migrated. For example, if migrating SQLite→PostgreSQL, do NOT touch WhatsApp services, API routes, middleware, UI components, or any file that doesn't directly reference the database.
3. NEVER replace real function implementations with placeholder stubs or "// Implementation would..." comments. Every function body in your output MUST be the COMPLETE, WORKING implementation.
4. NEVER truncate or simplify existing code. If a file has 500 lines and you only need to change 3 lines, your output must contain ALL 500 lines with only those 3 lines changed.
5. NEVER remove methods, classes, functions, or exports that exist in the original file unless they are DIRECTLY related to the old service being replaced.
6. If you are unsure whether a file needs changes, DO NOT include it in codeChanges. It is far better to miss a file than to destroy one.

## CODE QUALITY RULES:
1. For each file change, provide the COMPLETE new file content (not a diff). The content must be IDENTICAL to the original except for the specific lines that need to change for the migration.
2. Do not refactor unrelated code — only modify the specific lines that relate to the service connection.
3. Do not add unnecessary configuration options or connection pool tuning unless the original code had them.
4. Keep the same code style (semicolons, quotes, indentation) as the original file.
5. Do not add comments like "// PostgreSQL is now managed by LuxView Cloud" — the code should be self-explanatory.
6. Do not duplicate imports or code blocks — each import/statement should appear exactly once.
7. Files must end with a newline character.
8. The output file MUST have approximately the same number of lines as the input file (±20%). If your change reduces a file by more than 20%, you are almost certainly destroying code.

## BUILD & RUNTIME RULES (CRITICAL — violations cause runtime crashes):

### Node.js ESM Resolution:
1. If the project uses "type": "module" in package.json, Node.js ESM does NOT auto-resolve directory imports (e.g., import from './schema' will NOT find ./schema/index.js). You MUST either:
   a. Use a bundler like tsup that resolves these automatically, OR
   b. Add explicit /index.js extensions to all directory imports
2. If a package's package.json has "main" or "exports" pointing to ./src/index.ts (source TypeScript), it WILL FAIL at runtime because Node.js cannot import .ts files. Change them to point to ./dist/index.js (compiled output).
3. When changing a package to use a different build tool (e.g., tsc → tsup), update the "build" script AND add the new tool to devDependencies.

### Package Exports & Named Exports:
1. If a module uses 'import * as X from "./subdir"' internally but consumers import '{ X }' as a named export, you MUST add 'export { X }' explicitly.
2. 'export * from "./subdir"' re-exports individual symbols, NOT the namespace. If consumers use the namespace (e.g., { schema }), add an explicit namespace export.
3. When switching database drivers (e.g., libsql → postgres-js), the Drizzle ORM API changes:
   - libsql uses .all(), .get(), .values() methods
   - postgres-js uses direct await (no .all()/.get()/.values())
   - You MUST grep the entire codebase for .all(), .get(), .values() calls on db queries and remove them

### Monorepo Build Order:
1. In monorepos (pnpm workspaces, turborepo), packages MUST be built in dependency order: shared → database → app
2. If the Dockerfile only builds the app package, add build steps for dependency packages BEFORE the app build.
3. In the Dockerfile runtime stage, copy workspace packages from the 'builder' stage (which has compiled dist/), NOT from the 'deps' stage (which only has node_modules/).

### Drizzle ORM Migration Specifics (SQLite → PostgreSQL):
1. Replace 'drizzle-orm/libsql' or 'drizzle-orm/better-sqlite3' with 'drizzle-orm/postgres-js'
2. Replace '@libsql/client' or 'better-sqlite3' with 'postgres' (postgres.js driver)
3. Replace ALL 'sqliteTable' with 'pgTable' in schema files
4. Replace 'integer' type (SQLite) with 'serial' or 'integer' (PostgreSQL) for auto-increment primary keys
5. Replace 'text' mode: 'json' with 'jsonb()' for JSON columns
6. Update drizzle.config.ts: driver/dialect from 'sqlite'/'libsql' to 'postgresql', connection from file path to DATABASE_URL env var
7. Remove .all(), .get(), .values() from ALL query calls — postgres-js driver returns results directly with await

## DOCKERFILE RULES:
1. Do NOT add garbage lines or duplicate existing instructions.
2. Only modify Dockerfile lines that are directly related to the service change (e.g., removing SQLite directory setup).
3. Do NOT modify CMD, EXPOSE, or build stages unless directly required by the migration.

You MUST respond with valid JSON only. Use this exact format:
{
  "codeChanges": [
    {"file": "relative/path/to/file", "action": "modify|create|delete", "description": "What changed", "content": "full file content..."}
  ],
  "prTitle": "Short PR title describing the migration",
  "prBody": "Markdown body explaining what was changed and why"
}`

const buildFixSystemPrompt = `You are a Build Fix Agent for LuxView Cloud, a self-hosted PaaS platform.
A code migration was applied to a project but the build FAILED. Your job is to analyze the build error output,
the current state of the code, and generate ADDITIONAL code changes to fix the build errors.

## SECURITY RULES (MANDATORY):
1. NEVER hardcode credentials, passwords, connection strings, or secrets. Use ONLY environment variables.
2. NEVER add fallback values with real credentials. Fallbacks must be empty string or throw an error.
3. If an env var is required, the code MUST throw an error if it's missing.

## FIX RULES:
1. Generate ONLY the changes needed to fix the build errors. Do not refactor unrelated code.
2. For each file change, provide the COMPLETE new file content (not a diff).
3. Common issues to look for:
   - Missing imports after a module/package was replaced
   - Type mismatches after switching database drivers (e.g., sqliteTable vs pgTable in Drizzle ORM)
   - Files that still import removed dependencies (grep ALL source files, not just the main entry)
   - Missing peer dependencies in package.json
   - Incorrect module paths after package name changes
   - Schema files using the wrong DB-specific types
   - package.json "main"/"exports" pointing to ./src/*.ts instead of ./dist/*.js (crashes at runtime)
   - ESM directory imports without /index.js (ERR_UNSUPPORTED_DIR_IMPORT in Node.js)
   - Missing namespace exports (e.g., 'import * as schema' used internally but not re-exported as 'export { schema }')
   - Drizzle ORM .all()/.get()/.values() calls that don't exist in postgres-js driver (remove them, use direct await)
   - Monorepo Dockerfile not building dependency packages before the app package
4. If a file was not included in the original migration but imports a changed module, provide the updated file.
5. Preserve existing exports — if a file exported symbols, the new version must export the same symbols.
6. Do NOT introduce new exports that reference non-existent modules.
7. Files must end with a newline character.
8. Keep changes minimal — only fix what is broken.

You MUST respond with valid JSON only. Use this exact format:
{
  "codeChanges": [
    {"file": "relative/path/to/file", "action": "modify|create|delete", "description": "What was fixed", "content": "full file content..."}
  ]
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

	return sb.String(), nil
}

// BuildMigrationContext builds a user prompt for migration, reading ALL source code files
// so the AI agent can see every import, every usage, and generate a complete migration.
func BuildMigrationContext(repoDir string) (string, error) {
	log := logger.With("deploy-agent")

	tree, err := buildFileTree(repoDir)
	if err != nil {
		return "", fmt.Errorf("build file tree: %w", err)
	}

	// Read ALL source code files (not just key files + migration patterns)
	files, err := readAllSourceFiles(repoDir)
	if err != nil {
		log.Warn().Err(err).Msg("partial error reading source files")
	}

	log.Info().Int("files", len(files)).Msg("migration context: source files loaded")

	var sb strings.Builder
	sb.WriteString("## Repository File Tree\n```\n")
	sb.WriteString(tree)
	sb.WriteString("```\n\n")

	if len(files) > 0 {
		sb.WriteString("## Source Code (ALL files — you MUST check every file for imports/usages that need updating)\n\n")
		for name, content := range files {
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", name, content))
		}
	}

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

// readAllSourceFiles reads ALL source code files from the repo for full migration context.
// Uses maxMigrationContext (256KB) limit to fit within LLM context windows.
func readAllSourceFiles(repoDir string) (map[string]string, error) {
	log := logger.With("deploy-agent")
	files := make(map[string]string)
	totalSize := 0
	skippedCount := 0

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && skipSourceDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if totalSize >= maxMigrationContext {
			skippedCount++
			return nil
		}

		// Check if file extension is a source code file
		ext := strings.ToLower(filepath.Ext(info.Name()))
		fullName := strings.ToLower(info.Name())
		if !sourceCodeExtensions[ext] && !sourceCodeExtensions[fullName] {
			return nil
		}

		// Skip test files, lockfiles, and generated files to save context
		rel, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isSkippableForMigration(rel) {
			return nil
		}

		content, readErr := readFileLimited(path)
		if readErr != nil || len(content) == 0 {
			return nil
		}

		files[rel] = content
		totalSize += len(content)
		return nil
	})

	if skippedCount > 0 {
		log.Warn().Int("skipped", skippedCount).Int("total_kb", totalSize/1024).Msg("migration context limit reached, some files skipped")
	}

	return files, err
}

// isSkippableForMigration returns true for files that don't need to be in migration context.
func isSkippableForMigration(rel string) bool {
	lower := strings.ToLower(rel)
	// Skip test files
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "_test.go") || strings.HasPrefix(lower, "test/") ||
		strings.HasPrefix(lower, "tests/") || strings.HasPrefix(lower, "__tests__/") {
		return true
	}
	// Skip lockfiles (not source code)
	base := filepath.Base(lower)
	if base == "pnpm-lock.yaml" || base == "package-lock.json" || base == "yarn.lock" ||
		base == "poetry.lock" || base == "pipfile.lock" || base == "cargo.lock" ||
		base == "go.sum" {
		return true
	}
	// Skip generated/config files that don't contain import statements
	if base == "tsconfig.json" || base == "tsconfig.build.json" ||
		strings.HasPrefix(base, ".eslint") || strings.HasPrefix(base, ".prettier") {
		return true
	}
	return false
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
