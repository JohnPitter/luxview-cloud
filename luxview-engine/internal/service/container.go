package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/luxview/engine/internal/model"
	dockerclient "github.com/luxview/engine/pkg/docker"
	"github.com/luxview/engine/pkg/logger"
)

// ContainerManager manages Docker containers for user apps.
type ContainerManager struct {
	docker     *dockerclient.Client
	appNetwork string
}

func NewContainerManager(docker *dockerclient.Client, appNetwork string) *ContainerManager {
	return &ContainerManager{docker: docker, appNetwork: appNetwork}
}

// Start creates and starts a container for the given app.
// binds is an optional list of Docker bind mount strings (e.g., "/host/path:/container/path").
func (cm *ContainerManager) Start(ctx context.Context, app *model.App, imageTag string, envVars map[string]string, binds []string) (string, error) {
	log := logger.With("container")
	containerName := fmt.Sprintf("luxview-%s", app.Subdomain)

	cpuQuota, memory := parseResourceLimits(app.ResourceLimits)
	log.Debug().
		Str("container_name", containerName).
		Str("image_tag", imageTag).
		Int("env_vars", len(envVars)).
		Int("assigned_port", app.AssignedPort).
		Int("internal_port", app.InternalPort).
		Int64("cpu_nano", cpuQuota).
		Int64("memory_bytes", memory).
		Msg("starting container")

	// Build env var list
	var envList []string
	for k, v := range envVars {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	envList = append(envList, fmt.Sprintf("PORT=%d", app.InternalPort))

	portStr := strconv.Itoa(app.InternalPort)
	exposedPort := nat.Port(portStr + "/tcp")

	config := &container.Config{
		Image:        imageTag,
		Env:          envList,
		ExposedPorts: nat.PortSet{exposedPort: struct{}{}},
		Labels: map[string]string{
			"luxview.app":       app.Subdomain,
			"luxview.app.id":    app.ID.String(),
			"luxview.managed":   "true",
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			exposedPort: []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(app.AssignedPort)},
			},
		},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		Resources: container.Resources{
			NanoCPUs: cpuQuota,
			Memory:   memory,
		},
		Binds: binds,
	}

	networkConfig := &network.NetworkingConfig{}

	// Remove existing container with same name (if any)
	_ = cm.docker.StopContainer(ctx, containerName, 10)
	_ = cm.docker.RemoveContainer(ctx, containerName, true)
	log.Debug().Str("container_name", containerName).Msg("removed existing container with same name (if any)")

	containerID, err := cm.docker.CreateContainer(ctx, config, hostConfig, networkConfig, containerName)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if err := cm.docker.StartContainer(ctx, containerID); err != nil {
		_ = cm.docker.RemoveContainer(ctx, containerID, true)
		return "", fmt.Errorf("start container: %w", err)
	}

	// Connect to app network so containers can reach shared services (pg, redis, etc.)
	if cm.appNetwork != "" {
		if err := cm.docker.ConnectNetwork(ctx, cm.appNetwork, containerID); err != nil {
			log.Warn().Err(err).Str("network", cm.appNetwork).Msg("failed to connect container to app network")
		}
	}

	log.Info().Str("container", containerID[:12]).Str("app", app.Subdomain).Msg("container started")
	return containerID, nil
}

// Stop stops a running container.
func (cm *ContainerManager) Stop(ctx context.Context, containerID string) error {
	log := logger.With("container")
	if containerID == "" {
		return nil
	}
	log.Debug().Str("container_id", containerID[:min(12, len(containerID))]).Msg("stopping container")
	err := cm.docker.StopContainer(ctx, containerID, 30)
	if err != nil {
		log.Warn().Err(err).Str("container", containerID).Msg("failed to stop container")
		return err
	}
	log.Info().Str("container", containerID).Msg("container stopped")
	return nil
}

// Remove removes a container.
func (cm *ContainerManager) Remove(ctx context.Context, containerID string) error {
	log := logger.With("container")
	if containerID == "" {
		return nil
	}
	log.Debug().Str("container_id", containerID[:min(12, len(containerID))]).Msg("removing container")
	return cm.docker.RemoveContainer(ctx, containerID, true)
}

// Restart restarts a container.
func (cm *ContainerManager) Restart(ctx context.Context, containerID string) error {
	log := logger.With("container")
	if containerID == "" {
		return fmt.Errorf("no container to restart")
	}
	log.Debug().Str("container_id", containerID[:min(12, len(containerID))]).Msg("restarting container")
	return cm.docker.RestartContainer(ctx, containerID, 30)
}

// Logs returns the last N lines of container logs.
func (cm *ContainerManager) Logs(ctx context.Context, containerID string, tail string) (io.ReadCloser, error) {
	if containerID == "" {
		return nil, fmt.Errorf("no container ID")
	}
	return cm.docker.ContainerLogs(ctx, containerID, tail)
}

// LogsFollow returns a streaming reader that follows container logs in real time.
func (cm *ContainerManager) LogsFollow(ctx context.Context, containerID string, tail string, since string) (io.ReadCloser, error) {
	if containerID == "" {
		return nil, fmt.Errorf("no container ID")
	}
	return cm.docker.ContainerLogsFollow(ctx, containerID, tail, since)
}

// IsRunning checks if a container is running.
func (cm *ContainerManager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	if containerID == "" {
		return false, nil
	}
	info, err := cm.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}

// UpdateResources applies new resource limits to a running container without restart.
func (cm *ContainerManager) UpdateResources(ctx context.Context, containerID string, limits model.ResourceLimits) error {
	if containerID == "" {
		return fmt.Errorf("no container ID")
	}
	nanoCPUs, memory := parseResourceLimits(limits)
	log := logger.With("container")
	log.Info().Str("container", containerID[:min(12, len(containerID))]).Int64("cpu_nano", nanoCPUs).Int64("memory", memory).Msg("updating container resources")
	return cm.docker.UpdateContainerResources(ctx, containerID, container.UpdateConfig{
		Resources: container.Resources{
			NanoCPUs: nanoCPUs,
			Memory:   memory,
		},
	})
}

func parseResourceLimits(rl model.ResourceLimits) (nanoCPUs int64, memory int64) {
	// Parse CPU (e.g., "0.5" -> 500_000_000 nanocpus)
	if rl.CPU != "" {
		if cpu, err := strconv.ParseFloat(rl.CPU, 64); err == nil {
			nanoCPUs = int64(cpu * 1e9)
		}
	}
	if nanoCPUs == 0 {
		nanoCPUs = 500_000_000 // default 0.5 CPU
	}

	// Parse memory (e.g., "512m" -> 512 * 1024 * 1024)
	if rl.Memory != "" {
		memory = parseMemory(rl.Memory)
	}
	if memory == 0 {
		memory = 512 * 1024 * 1024 // default 512MB
	}

	return nanoCPUs, memory
}

func parseMemory(s string) int64 {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}

	suffix := s[len(s)-1:]
	numStr := s[:len(s)-1]

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}

	switch suffix {
	case "g", "G":
		return num * 1024 * 1024 * 1024
	case "m", "M":
		return num * 1024 * 1024
	case "k", "K":
		return num * 1024
	default:
		// Try parsing the whole string as bytes
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
		return 0
	}
}
