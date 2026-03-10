package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Client wraps the Docker SDK client.
type Client struct {
	cli *client.Client
}

// New creates a new Docker client from the environment.
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

// Close closes the underlying client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Raw returns the underlying Docker client for advanced usage.
func (c *Client) Raw() *client.Client {
	return c.cli
}

// BuildImage builds a Docker image from a tar context.
func (c *Client) BuildImage(ctx context.Context, buildContext io.Reader, tags []string, dockerfile string) (io.ReadCloser, error) {
	opts := types.ImageBuildOptions{
		Tags:       tags,
		Dockerfile: dockerfile,
		Remove:     true,
		ForceRemove: true,
	}
	resp, err := c.cli.ImageBuild(ctx, buildContext, opts)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// CreateContainer creates a container with the given configuration.
func (c *Client) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkConfig *network.NetworkingConfig, name string) (string, error) {
	resp, err := c.cli.ContainerCreate(ctx, config, hostConfig, networkConfig, &ocispec.Platform{}, name)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// StartContainer starts a container by ID.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a container with a timeout.
func (c *Client) StopContainer(ctx context.Context, containerID string, timeoutSec int) error {
	timeout := time.Duration(timeoutSec) * time.Second
	opts := container.StopOptions{Timeout: intPtr(int(timeout.Seconds()))}
	return c.cli.ContainerStop(ctx, containerID, opts)
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force, RemoveVolumes: true})
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, containerID string, timeoutSec int) error {
	opts := container.StopOptions{Timeout: intPtr(timeoutSec)}
	return c.cli.ContainerRestart(ctx, containerID, opts)
}

// InspectContainer returns container details.
func (c *Client) InspectContainer(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return c.cli.ContainerInspect(ctx, containerID)
}

// ContainerLogs returns container logs as a reader.
func (c *Client) ContainerLogs(ctx context.Context, containerID string, tail string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: true,
	})
}

// ContainerLogsFollow returns a streaming reader that follows container logs in real time.
func (c *Client) ContainerLogsFollow(ctx context.Context, containerID string, tail string, since string) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       tail,
	}
	if since != "" {
		opts.Since = since
	}
	return c.cli.ContainerLogs(ctx, containerID, opts)
}

// ContainerStats returns a single stats snapshot for a container.
func (c *Client) ContainerStats(ctx context.Context, containerID string) (container.StatsResponseReader, error) {
	return c.cli.ContainerStats(ctx, containerID, false)
}

// RemoveImage removes an image by tag or ID.
func (c *Client) RemoveImage(ctx context.Context, imageID string) error {
	_, err := c.cli.ImageRemove(ctx, imageID, image.RemoveOptions{Force: true, PruneChildren: true})
	return err
}

// ListContainers lists all containers (running and stopped).
func (c *Client) ListContainers(ctx context.Context) ([]types.Container, error) {
	return c.cli.ContainerList(ctx, container.ListOptions{All: true})
}

// ConnectNetwork connects a container to a network.
func (c *Client) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	return c.cli.NetworkConnect(ctx, networkID, containerID, nil)
}

// ContainerExec runs a command inside a running container and returns the output.
func (c *Client) ContainerExec(ctx context.Context, containerID string, cmd []string) (string, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := c.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", err
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attachResp.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, attachResp.Reader)

	// Check exit code
	inspectResp, err := c.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return buf.String(), err
	}

	if inspectResp.ExitCode != 0 {
		return buf.String(), fmt.Errorf("exec exited with code %d", inspectResp.ExitCode)
	}

	return buf.String(), nil
}

// PruneResult holds the result of a system-wide prune operation.
type PruneResult struct {
	ImagesRemoved      int   `json:"imagesRemoved"`
	ContainersRemoved  int   `json:"containersRemoved"`
	BuildCacheReclaimed int64 `json:"buildCacheReclaimed"`
	ImagesReclaimed    int64 `json:"imagesReclaimed"`
	TotalReclaimed     int64 `json:"totalReclaimed"`
}

// SystemPrune removes unused containers, dangling images, and build cache.
func (c *Client) SystemPrune(ctx context.Context) (*PruneResult, error) {
	result := &PruneResult{}

	// Prune stopped containers
	containerReport, err := c.cli.ContainersPrune(ctx, filters.Args{})
	if err == nil {
		result.ContainersRemoved = len(containerReport.ContainersDeleted)
		result.TotalReclaimed += int64(containerReport.SpaceReclaimed)
	}

	// Prune all unused images (not just dangling)
	pruneFilters := filters.NewArgs()
	pruneFilters.Add("dangling", "false")
	imageReport, err := c.cli.ImagesPrune(ctx, pruneFilters)
	if err == nil {
		result.ImagesRemoved = len(imageReport.ImagesDeleted)
		result.ImagesReclaimed = int64(imageReport.SpaceReclaimed)
		result.TotalReclaimed += int64(imageReport.SpaceReclaimed)
	}

	// Prune build cache
	buildReport, err := c.cli.BuildCachePrune(ctx, types.BuildCachePruneOptions{All: true})
	if err == nil {
		result.BuildCacheReclaimed = int64(buildReport.SpaceReclaimed)
		result.TotalReclaimed += int64(buildReport.SpaceReclaimed)
	}

	return result, nil
}

func intPtr(i int) *int {
	return &i
}
