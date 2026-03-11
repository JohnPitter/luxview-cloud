package service

import (
	"os"
	"path/filepath"

	"github.com/luxview/engine/internal/buildpack"
	"github.com/luxview/engine/pkg/logger"
)

// Detector detects the stack of a cloned repository.
type Detector struct{}

func NewDetector() *Detector {
	return &Detector{}
}

// DetectResult holds the detected buildpack and the directory to build from.
type DetectResult struct {
	Buildpack buildpack.Buildpack
	BuildDir  string
}

// Detect scans the source directory and returns the matching buildpack and build directory.
// Returns nil for monorepos (turbo/pnpm/lerna/nx) since workspace:* cross-dependencies
// require a custom Dockerfile. For non-monorepos, searches one level deep if no match at root.
func (d *Detector) Detect(sourceDir string) *DetectResult {
	log := logger.With("detector")

	files, err := listRootEntries(sourceDir)
	if err != nil {
		log.Error().Err(err).Str("dir", sourceDir).Msg("failed to list source directory")
		return nil
	}

	log.Debug().Int("file_count", len(files)).Msg("root files found in source directory")
	for _, f := range files {
		log.Debug().Str("file", f).Msg("root file")
	}

	// Monorepos (turbo/pnpm/lerna/nx) have workspace:* cross-dependencies that
	// generic buildpacks can't resolve. They always need a custom Dockerfile.
	if d.IsMonorepo(sourceDir) {
		log.Warn().Str("dir", sourceDir).Msg("monorepo detected — auto-detect not supported, requires AI analysis or custom Dockerfile")
		return nil
	}

	bp := buildpack.DetectStack(files)
	if bp != nil {
		log.Info().Str("stack", bp.Name()).Str("dir", sourceDir).Msg("stack detected")
		return &DetectResult{Buildpack: bp, BuildDir: sourceDir}
	}

	// No match at root — check subdirectories for a buildable project
	entries, _ := os.ReadDir(sourceDir)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".git" || e.Name() == "node_modules" || e.Name() == ".github" {
			continue
		}
		subDir := filepath.Join(sourceDir, e.Name())
		subFiles, err := listRootEntries(subDir)
		if err != nil {
			continue
		}
		subBp := buildpack.DetectStack(subFiles)
		matched := subBp != nil
		log.Debug().Str("subdir", e.Name()).Bool("matched", matched).Msg("checking subdirectory")
		if subBp != nil {
			log.Info().Str("stack", subBp.Name()).Str("subdir", e.Name()).Msg("stack detected in subdirectory")
			return &DetectResult{Buildpack: subBp, BuildDir: subDir}
		}
	}

	log.Warn().Str("dir", sourceDir).Msg("no buildpack detected")
	return nil
}

// isMonorepo checks if the source directory is a monorepo (turbo, pnpm workspaces, lerna, nx).
func (d *Detector) IsMonorepo(sourceDir string) bool {
	monorepoMarkers := []string{
		"turbo.json",
		"pnpm-workspace.yaml",
		"lerna.json",
		"nx.json",
	}
	for _, marker := range monorepoMarkers {
		if _, err := os.Stat(filepath.Join(sourceDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// listRootEntries returns filenames (not dirs) in the root of the directory.
func listRootEntries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Base(e.Name()))
	}
	return files, nil
}
