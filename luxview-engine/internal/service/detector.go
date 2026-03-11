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
// For monorepos (turbo/pnpm/lerna), it skips generic NodePack at root and instead
// searches packages/ and apps/ subdirectories for a deployable backend service.
// If no buildpack is found at root level, it searches one level deep for a
// buildable subdirectory (e.g., monorepos with a single service in a subfolder).
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

	// Check if this is a monorepo before running generic detection
	isMonorepo := d.isMonorepo(sourceDir)
	if isMonorepo {
		log.Info().Str("dir", sourceDir).Msg("monorepo detected, scanning workspaces for deployable service")
		result := d.detectMonorepoService(sourceDir)
		if result != nil {
			return result
		}
		log.Warn().Str("dir", sourceDir).Msg("monorepo detected but no deployable service found in workspaces — requires custom Dockerfile")
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
func (d *Detector) isMonorepo(sourceDir string) bool {
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

// detectMonorepoService scans monorepo workspace directories to find a deployable
// backend service. It prioritizes backend apps (api, server, backend) over frontend.
// Returns nil if no suitable service is found (monorepo needs a custom Dockerfile).
func (d *Detector) detectMonorepoService(sourceDir string) *DetectResult {
	log := logger.With("detector")

	// Directories to scan for workspace packages
	workspaceDirs := []string{"apps", "packages", "services"}

	// Priority names for backend services (checked first)
	backendNames := map[string]bool{
		"api": true, "server": true, "backend": true,
		"service": true, "web-api": true, "app": true,
	}

	var frontendResult *DetectResult

	for _, wsDir := range workspaceDirs {
		wsPath := filepath.Join(sourceDir, wsDir)
		entries, err := os.ReadDir(wsPath)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			subDir := filepath.Join(wsPath, e.Name())
			subFiles, err := listRootEntries(subDir)
			if err != nil {
				continue
			}

			subBp := buildpack.DetectStack(subFiles)
			if subBp == nil {
				log.Debug().Str("workspace", wsDir+"/"+e.Name()).Msg("no buildpack detected in workspace package")
				continue
			}

			log.Debug().
				Str("workspace", wsDir+"/"+e.Name()).
				Str("stack", subBp.Name()).
				Bool("is_backend_name", backendNames[e.Name()]).
				Msg("buildpack detected in workspace package")

			// Skip Vite/static SPAs as primary deploy target — they need a combined Dockerfile
			isFrontend := subBp.Name() == "vite" || subBp.Name() == "static"

			if backendNames[e.Name()] && !isFrontend {
				log.Info().
					Str("stack", subBp.Name()).
					Str("workspace", wsDir+"/"+e.Name()).
					Msg("backend service detected in monorepo")
				return &DetectResult{Buildpack: subBp, BuildDir: subDir}
			}

			// Remember first non-frontend match as fallback
			if !isFrontend && frontendResult == nil {
				frontendResult = &DetectResult{Buildpack: subBp, BuildDir: subDir}
			}
		}
	}

	// Fallback: use the first non-frontend service found
	if frontendResult != nil {
		log.Info().
			Str("stack", frontendResult.Buildpack.Name()).
			Str("dir", frontendResult.BuildDir).
			Msg("using fallback non-frontend service from monorepo")
		return frontendResult
	}

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
