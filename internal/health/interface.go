package health

import (
	"context"
	"time"

	"github.com/bpradana/sentinel/internal/config"
)

// Status represents the health status of a target
type Status int

const (
	StatusUnknown Status = iota
	StatusHealthy
	StatusUnhealthy
)

func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// TargetHealth represents the health state of a target
type TargetHealth struct {
	URL                  string
	Status               Status
	LastCheck            time.Time
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	ResponseTime         time.Duration
	Error                error
}

// Checker defines the interface for health checking
type Checker interface {
	// Start starts the health checker
	Start()
	// Stop stops the health checker
	Stop()
	// CheckTarget performs a health check on a target
	CheckTarget(ctx context.Context, url string, config config.HealthCheckConfig) *TargetHealth
	// IsHealthy returns whether a target is healthy
	IsHealthy(url string) bool
	// GetHealth returns the health status of a target
	GetHealth(url string) *TargetHealth
	// GetAllHealth returns the health status of all targets
	GetAllHealth() map[string]*TargetHealth
}
