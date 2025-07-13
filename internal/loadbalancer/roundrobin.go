package loadbalancer

import (
	"errors"
	"net/http"
	"sync"
)

// RoundRobin implements round-robin load balancing
type RoundRobin struct {
	mu      sync.Mutex
	current int
}

// NewRoundRobin creates a new round-robin load balancer
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

// SelectTarget selects the next target in round-robin fashion
func (rr *RoundRobin) SelectTarget(targets []*Target, req *http.Request) (*Target, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

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

	// Select target using round-robin
	target := healthyTargets[rr.current%len(healthyTargets)]
	rr.current++

	return target, nil
}

// UpdateTarget updates target state (no-op for round-robin)
func (rr *RoundRobin) UpdateTarget(target *Target, delta int) {
	// Round-robin doesn't need to track connection state
}

// Name returns the name of the strategy
func (rr *RoundRobin) Name() string {
	return "round_robin"
}
