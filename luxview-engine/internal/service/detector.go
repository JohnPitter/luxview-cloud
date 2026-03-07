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
// If no buildpack is found at root level, it searches one level deep for a
// buildable subdirectory (e.g., monorepos with a single service in a subfolder).
func (d *Detector) Detect(sourceDir string) *DetectResult {
	log := logger.With("detector")

	files, err := listRootEntries(sourceDir)
	if err != nil {
		log.Error().Err(err).Str("dir", sourceDir).Msg("failed to list source directory")
		return nil
	}

	log.Debug().Strs("files", files).Msg("scanned source directory")

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
		if subBp != nil {
			log.Info().Str("stack", subBp.Name()).Str("subdir", e.Name()).Msg("stack detected in subdirectory")
			return &DetectResult{Buildpack: subBp, BuildDir: subDir}
		}
	}

	log.Warn().Str("dir", sourceDir).Msg("no buildpack detected")
	return nil
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
