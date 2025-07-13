package loadbalancer

import (
	"errors"
	"hash/fnv"
	"net"
	"net/http"
	"strings"
)

// IPHash implements IP hash load balancing
type IPHash struct{}

// NewIPHash creates a new IP hash load balancer
func NewIPHash() *IPHash {
	return &IPHash{}
}

// SelectTarget selects a target based on client IP hash
func (ih *IPHash) SelectTarget(targets []*Target, req *http.Request) (*Target, error) {
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

	// Get client IP
	clientIP := ih.getClientIP(req)

	// Hash the IP
	hash := ih.hashIP(clientIP)

	// Select target based on hash
	index := hash % uint32(len(healthyTargets))
	return healthyTargets[index], nil
}

// UpdateTarget updates target state (no-op for IP hash)
func (ih *IPHash) UpdateTarget(target *Target, delta int) {
	// IP hash doesn't need to track connection state
}

// Name returns the name of the strategy
func (ih *IPHash) Name() string {
	return "ip_hash"
}

// getClientIP extracts the client IP from the request
func (ih *IPHash) getClientIP(req *http.Request) string {
	// Check X-Real-IP header first
	if ip := req.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP from the list
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

// hashIP creates a hash of the IP address
func (ih *IPHash) hashIP(ip string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return h.Sum32()
}
