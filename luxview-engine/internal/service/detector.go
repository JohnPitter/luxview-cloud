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

// Detect scans the source directory and returns the matching buildpack.
func (d *Detector) Detect(sourceDir string) buildpack.Buildpack {
	log := logger.With("detector")

	files, err := listRootFiles(sourceDir)
	if err != nil {
		log.Error().Err(err).Str("dir", sourceDir).Msg("failed to list source directory")
		return nil
	}

	log.Debug().Strs("files", files).Msg("scanned source directory")

	bp := buildpack.DetectStack(files)
	if bp == nil {
		log.Warn().Str("dir", sourceDir).Msg("no buildpack detected")
		return nil
	}

	log.Info().Str("stack", bp.Name()).Str("dir", sourceDir).Msg("stack detected")
	return bp
}

// listRootFiles returns the filenames (not full paths) in the root of the directory.
func listRootFiles(dir string) ([]string, error) {
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
