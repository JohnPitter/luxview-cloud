package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

func intPtr(i int) *int {
	return &i
}
