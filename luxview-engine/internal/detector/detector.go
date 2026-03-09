package detector

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

// Detection holds the detected runtime, framework, and default port.
type Detection struct {
	Runtime   string
	Framework string
	Port      int
}

// Analyze performs deterministic analysis of a repository directory
// and returns an AnalysisResult without requiring AI.
func Analyze(repoDir string) *agent.AnalysisResult {
	det := detect(repoDir)
	envVars := detectEnvVars(repoDir)
	services := detectServices(repoDir)
	dockerfile := generateDockerfile(det, repoDir)

	return &agent.AnalysisResult{
		Suggestions:            []agent.Suggestion{},
		Dockerfile:             dockerfile,
		Port:                   det.Port,
		Stack:                  det.Runtime,
		EnvHints:               envVars,
		ServiceRecommendations: services,
	}
}

func detect(repoDir string) Detection {
	if fileExists(repoDir, "package.json") {
		pkg := readFile(repoDir, "package.json")
		if strings.Contains(pkg, "\"next\"") {
			return Detection{Runtime: "nodejs", Framework: "nextjs", Port: 3000}
		}
		if strings.Contains(pkg, "\"vite\"") || strings.Contains(pkg, "\"@vitejs/") {
			return Detection{Runtime: "nodejs", Framework: "vite", Port: 80}
		}
		if strings.Contains(pkg, "\"express\"") {
			return Detection{Runtime: "nodejs", Framework: "express", Port: 3000}
		}
		if strings.Contains(pkg, "\"@nestjs/core\"") {
			return Detection{Runtime: "nodejs", Framework: "nestjs", Port: 3000}
		}
		if strings.Contains(pkg, "\"fastify\"") {
			return Detection{Runtime: "nodejs", Framework: "fastify", Port: 3000}
		}
		return Detection{Runtime: "nodejs", Framework: "node", Port: 3000}
	}

	if fileExists(repoDir, "requirements.txt") || fileExists(repoDir, "pyproject.toml") || fileExists(repoDir, "Pipfile") {
		if fileExists(repoDir, "manage.py") {
			return Detection{Runtime: "python", Framework: "django", Port: 8000}
		}
		content := readFile(repoDir, "requirements.txt") + readFile(repoDir, "pyproject.toml")
		if strings.Contains(content, "fastapi") {
			return Detection{Runtime: "python", Framework: "fastapi", Port: 8000}
		}
		if strings.Contains(content, "flask") {
			return Detection{Runtime: "python", Framework: "flask", Port: 5000}
		}
		return Detection{Runtime: "python", Framework: "python", Port: 8000}
	}

	if fileExists(repoDir, "go.mod") {
		content := readFile(repoDir, "go.mod")
		if strings.Contains(content, "github.com/gin-gonic/gin") {
			return Detection{Runtime: "go", Framework: "gin", Port: 8080}
		}
		if strings.Contains(content, "github.com/gofiber/fiber") {
			return Detection{Runtime: "go", Framework: "fiber", Port: 8080}
		}
		return Detection{Runtime: "go", Framework: "go", Port: 8080}
	}

	if fileExists(repoDir, "Gemfile") {
		content := readFile(repoDir, "Gemfile")
		if strings.Contains(content, "rails") {
			return Detection{Runtime: "ruby", Framework: "rails", Port: 3000}
		}
		return Detection{Runtime: "ruby", Framework: "ruby", Port: 3000}
	}

	if fileExists(repoDir, "pom.xml") {
		return Detection{Runtime: "java", Framework: "maven", Port: 8080}
	}
	if fileExists(repoDir, "build.gradle") || fileExists(repoDir, "build.gradle.kts") {
		return Detection{Runtime: "java", Framework: "gradle", Port: 8080}
	}

	if fileExists(repoDir, "Cargo.toml") {
		return Detection{Runtime: "rust", Framework: "rust", Port: 8080}
	}

	if fileExists(repoDir, "index.html") {
		return Detection{Runtime: "static", Framework: "static", Port: 80}
	}

	return Detection{Runtime: "unknown", Framework: "unknown", Port: 3000}
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func readFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	if len(data) > 32*1024 {
		data = data[:32*1024]
	}
	return string(data)
}
