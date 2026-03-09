package detector

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luxview/engine/internal/agent"
)

var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`os\.environ\["([A-Z_][A-Z0-9_]*)"\]`),
	regexp.MustCompile(`os\.environ\.get\("([A-Z_][A-Z0-9_]*)"`),
	regexp.MustCompile(`os\.Getenv\("([A-Z_][A-Z0-9_]*)"\)`),
	regexp.MustCompile(`env\("([A-Z_][A-Z0-9_]*)"\)`),
}

var platformEnvVars = map[string]bool{
	"NODE_ENV": true, "PORT": true, "HOME": true, "PATH": true,
	"CI": true, "PWD": true, "USER": true, "SHELL": true,
	"HOSTNAME": true, "TERM": true, "LANG": true,
}

func detectEnvVars(repoDir string) []agent.EnvHint {
	seen := make(map[string]bool)
	var hints []agent.EnvHint

	for _, name := range []string{".env.example", ".env.sample", ".env.template"} {
		path := filepath.Join(repoDir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key == "" || seen[key] || platformEnvVars[key] {
				continue
			}
			seen[key] = true
			hints = append(hints, agent.EnvHint{
				Key:         key,
				Description: "From " + name,
				Required:    true,
			})
		}
		f.Close()
	}

	scanDir(repoDir, repoDir, seen, &hints)

	return hints
}

func scanDir(baseDir, dir string, seen map[string]bool, hints *[]agent.EnvHint) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, "dist": true, "build": true,
		".next": true, "__pycache__": true, "vendor": true, ".venv": true,
		"target": true, "coverage": true,
	}

	for _, e := range entries {
		if e.IsDir() {
			if skipDirs[e.Name()] {
				continue
			}
			scanDir(baseDir, filepath.Join(dir, e.Name()), seen, hints)
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" &&
			ext != ".py" && ext != ".go" && ext != ".rs" && ext != ".java" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(data) > 64*1024 {
			continue
		}
		content := string(data)

		for _, re := range envPatterns {
			matches := re.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				key := m[1]
				if seen[key] || platformEnvVars[key] {
					continue
				}
				seen[key] = true
				*hints = append(*hints, agent.EnvHint{
					Key:         key,
					Description: "Referenced in source code",
					Required:    false,
				})
			}
		}
	}
}
