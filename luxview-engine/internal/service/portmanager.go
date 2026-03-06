package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// PortManager allocates and releases ports for containers.
type PortManager struct {
	mu       sync.Mutex
	appRepo  *repository.AppRepo
	start    int
	end      int
	reserved map[int]bool
}

func NewPortManager(appRepo *repository.AppRepo, start, end int) *PortManager {
	return &PortManager{
		appRepo:  appRepo,
		start:    start,
		end:      end,
		reserved: make(map[int]bool),
	}
}

// Allocate finds the next available port in the range.
func (pm *PortManager) Allocate(ctx context.Context) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	log := logger.With("portmanager")

	used, err := pm.appRepo.GetUsedPorts(ctx)
	if err != nil {
		return 0, fmt.Errorf("get used ports: %w", err)
	}

	// Merge with locally reserved ports
	for p := range pm.reserved {
		used[p] = true
	}

	for port := pm.start; port <= pm.end; port++ {
		if !used[port] {
			pm.reserved[port] = true
			log.Info().Int("port", port).Msg("port allocated")
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", pm.start, pm.end)
}

// Release removes a port from the local reservation cache.
func (pm *PortManager) Release(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.reserved, port)
}
