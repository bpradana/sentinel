package loadbalancer

import (
	"errors"
	"net/http"
	"sync"
)

// LeastConnections implements least connections load balancing
type LeastConnections struct {
	mu sync.Mutex
}

// NewLeastConnections creates a new least connections load balancer
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

// SelectTarget selects the target with the least connections
func (lc *LeastConnections) SelectTarget(targets []*Target, req *http.Request) (*Target, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if len(targets) == 0 {
		return nil, errors.New("no targets available")
	}

	// Filter healthy targets
	healthyTargets := make([]*Target, 0, len(targets))
	for _, target := range targets {
		if target.IsHealthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	if len(healthyTargets) == 0 {
		return nil, errors.New("no healthy targets available")
	}

	// Find target with least connections
	var selected *Target
	minConnections := -1

	for _, target := range healthyTargets {
		if minConnections == -1 || target.Connections < minConnections {
			minConnections = target.Connections
			selected = target
		}
	}

	return selected, nil
}

// UpdateTarget updates the connection count for a target
func (lc *LeastConnections) UpdateTarget(target *Target, delta int) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	target.Connections += delta
	if target.Connections < 0 {
		target.Connections = 0
	}
}

// Name returns the name of the strategy
func (lc *LeastConnections) Name() string {
	return "least_connections"
}
