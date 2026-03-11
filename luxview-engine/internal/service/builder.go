package service

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxview/engine/internal/buildpack"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// Builder builds Docker images from source directories.
type Builder struct {
	docker *dockerclient.Client
}

func NewBuilder(docker *dockerclient.Client) *Builder {
	return &Builder{docker: docker}
}

// Build creates a Docker image from the source directory using the given buildpack.
// Returns the build log output.
func (b *Builder) Build(ctx context.Context, sourceDir string, bp buildpack.Buildpack, imageTag string) (string, error) {
	log := logger.With("builder")
	log.Info().Str("image", imageTag).Str("stack", bp.Name()).Msg("starting build")

	// Generate Dockerfile if not using the repo's own
	dockerfileContent := bp.Dockerfile(sourceDir)
	dockerfileName := "Dockerfile"

	if dockerfileContent != "" {
		dfPreview := dockerfileContent
		if len(dfPreview) > 200 {
			dfPreview = dfPreview[:200]
		}
		log.Debug().Str("dockerfile_preview", dfPreview).Msg("generated Dockerfile content")

		// Write generated Dockerfile to source dir
		dfPath := filepath.Join(sourceDir, "Dockerfile.luxview")
		if err := os.WriteFile(dfPath, []byte(dockerfileContent), 0644); err != nil {
			return "", fmt.Errorf("write generated dockerfile: %w", err)
		}
		dockerfileName = "Dockerfile.luxview"
		defer os.Remove(dfPath)
	}

	// Create tar archive of source directory
	tarBuf, err := createTarArchive(sourceDir)
	if err != nil {
		return "", fmt.Errorf("create tar archive: %w", err)
	}

	log.Debug().Int("tar_size_bytes", tarBuf.Len()).Str("dockerfile", dockerfileName).Msg("tar archive created, sending to Docker")

	// Build the image
	resp, err := b.docker.BuildImage(ctx, tarBuf, []string{imageTag}, dockerfileName)
	if err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	defer resp.Close()

	// Read build output and check for errors
	var buildLog strings.Builder
	decoder := json.NewDecoder(resp)
	var lastErr string
	lineCount := 0
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return buildLog.String(), fmt.Errorf("read build output: %w", err)
		}
		if msg.Stream != "" {
			buildLog.WriteString(msg.Stream)
			lineCount++
			linePreview := msg.Stream
			if len(linePreview) > 200 {
				linePreview = linePreview[:200]
			}
			log.Debug().Str("image", imageTag).Int("line", lineCount).Str("output", linePreview).Msg("build stream")
		}
		if msg.Error != "" {
			lastErr = msg.Error
			buildLog.WriteString("ERROR: " + msg.Error + "\n")
		}
	}

	if lastErr != "" {
		log.Error().Str("image", imageTag).Str("error", lastErr).Msg("build failed")
		return buildLog.String(), fmt.Errorf("docker build failed: %s", lastErr)
	}

	log.Info().Str("image", imageTag).Msg("build completed")
	return buildLog.String(), nil
}

// createTarArchive creates a tar archive from a directory.
func createTarArchive(sourceDir string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip node_modules
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	return buf, err
}
