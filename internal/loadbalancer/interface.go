package loadbalancer

import (
	"net/http"
	"net/url"
)

// Target represents an upstream target
type Target struct {
	URL         *url.URL
	Weight      int
	IsHealthy   bool
	Connections int
}

// LoadBalancer defines the interface for load balancing strategies
type LoadBalancer interface {
	// SelectTarget selects a target based on the load balancing strategy
	SelectTarget(targets []*Target, req *http.Request) (*Target, error)
	// UpdateTarget updates target state (e.g., connection count)
	UpdateTarget(target *Target, delta int)
	// Name returns the name of the load balancing strategy
	Name() string
}

// Factory creates load balancers
type Factory interface {
	Create(strategy string) (LoadBalancer, error)
}

// DefaultFactory is the default load balancer factory
type DefaultFactory struct{}

// Create creates a load balancer based on the strategy
func (f *DefaultFactory) Create(strategy string) (LoadBalancer, error) {
	switch strategy {
	case "round_robin":
		return NewRoundRobin(), nil
	case "least_connections":
		return NewLeastConnections(), nil
	case "ip_hash":
		return NewIPHash(), nil
	default:
		return NewRoundRobin(), nil // Default to round robin
	}
}
