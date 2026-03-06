package service

import (
	"archive/tar"
	"bytes"
	"context"
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

	// Build the image
	resp, err := b.docker.BuildImage(ctx, tarBuf, []string{imageTag}, dockerfileName)
	if err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	defer resp.Close()

	// Read build output
	var buildLog strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Read(buf)
		if n > 0 {
			buildLog.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return buildLog.String(), fmt.Errorf("read build output: %w", readErr)
		}
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
