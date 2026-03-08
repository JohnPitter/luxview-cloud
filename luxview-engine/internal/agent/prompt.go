package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxview/engine/pkg/logger"
)

const maxFilesInTree = 200
const maxFileSize = 16 * 1024  // 16KB per file
const maxTotalContext = 50 * 1024 // 50KB total context

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
}

// Monorepo glob patterns for additional package.json files.
var monorepoPatterns = []string{
	"apps/*/package.json",
	"packages/*/package.json",
	"apps/*/Dockerfile",
	"packages/*/Dockerfile",
}

const systemPrompt = `You are a Deploy Agent for LuxView Cloud, a self-hosted PaaS platform.
Your job is to analyze a user's repository and generate an optimal Dockerfile for deployment.

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
5. For monorepos, bundle everything into a single container. Identify the main application entry point.
6. Use .dockerignore best practices (node_modules, .git, etc. are already excluded).
7. Install only production dependencies when possible.
8. Set appropriate WORKDIR, COPY, and CMD instructions.

You MUST respond with valid JSON only (no markdown, no explanation outside JSON). Use this exact format:
{
  "suggestions": [{"type": "error|warning|info", "message": "..."}],
  "dockerfile": "FROM ...\n...",
  "port": 3000,
  "stack": "nodejs|nextjs|vite|python|go|java|rust|static",
  "envHints": [{"key": "DATABASE_URL", "description": "...", "required": true}]
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

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
