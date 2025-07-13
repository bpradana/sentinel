package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bpradana/sentinel/internal/config"
	"go.uber.org/zap"
)

// checker implements the Checker interface
type checker struct {
	cfg    config.HealthConfig
	logger *zap.Logger
	client *http.Client
	
	// State management
	targets map[string]*TargetHealth
	mu      sync.RWMutex
	
	// Control channels
	stopCh chan struct{}
	done   chan struct{}
}

// NewChecker creates a new health checker instance
func NewChecker(cfg config.HealthConfig, logger *zap.Logger) Checker {
	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			MaxIdleConns:        1,
			MaxIdleConnsPerHost: 1,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &checker{
		cfg:     cfg,
		logger:  logger,
		client:  client,
		targets: make(map[string]*TargetHealth),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start starts the health checker
func (c *checker) Start() {
	if !c.cfg.Enabled {
		c.logger.Info("Health checker disabled")
		close(c.done)
		return
	}

	c.logger.Info("Starting health checker", 
		zap.Duration("interval", c.cfg.Interval),
		zap.Duration("timeout", c.cfg.Timeout))

	go c.run()
}

// Stop stops the health checker
func (c *checker) Stop() {
	c.logger.Info("Stopping health checker")
	close(c.stopCh)
	<-c.done
}

// run is the main health checking loop
func (c *checker) run() {
	defer close(c.done)
	
	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all registered targets
func (c *checker) performHealthChecks() {
	c.mu.RLock()
	targets := make(map[string]*TargetHealth, len(c.targets))
	for url, health := range c.targets {
		targets[url] = health
	}
	c.mu.RUnlock()

	var wg sync.WaitGroup
	for url := range targets {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			
			// Create a default health check config if not available
			healthConfig := config.HealthCheckConfig{
				Enabled:          true,
				Path:             "/health",
				Interval:         c.cfg.Interval,
				Timeout:          c.cfg.Timeout,
				FailureThreshold: 3,
				SuccessThreshold: 2,
			}
			
			health := c.CheckTarget(context.Background(), targetURL, healthConfig)
			
			c.mu.Lock()
			c.targets[targetURL] = health
			c.mu.Unlock()
		}(url)
	}
	
	wg.Wait()
}

// CheckTarget performs a health check on a target
func (c *checker) CheckTarget(ctx context.Context, url string, config config.HealthCheckConfig) *TargetHealth {
	if !config.Enabled {
		return &TargetHealth{
			URL:         url,
			Status:      StatusHealthy, // Assume healthy if checks disabled
			LastCheck:   time.Now(),
			Error:       nil,
		}
	}

	start := time.Now()
	
	// Get existing health state
	c.mu.RLock()
	existing := c.targets[url]
	c.mu.RUnlock()
	
	if existing == nil {
		existing = &TargetHealth{
			URL:    url,
			Status: StatusUnknown,
		}
	}

	// Construct health check URL
	healthURL := url
	if config.Path != "" {
		healthURL = fmt.Sprintf("%s%s", url, config.Path)
	}

	// Create request with timeout
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return c.updateTargetHealth(existing, false, time.Since(start), fmt.Errorf("failed to create request: %w", err), config)
	}

	// Perform health check
	resp, err := c.client.Do(req)
	if err != nil {
		return c.updateTargetHealth(existing, false, time.Since(start), fmt.Errorf("health check failed: %w", err), config)
	}
	defer resp.Body.Close()

	responseTime := time.Since(start)
	
	// Check response status
	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	var healthErr error
	if !isHealthy {
		healthErr = fmt.Errorf("unhealthy status code: %d", resp.StatusCode)
	}

	return c.updateTargetHealth(existing, isHealthy, responseTime, healthErr, config)
}

// updateTargetHealth updates the health state of a target
func (c *checker) updateTargetHealth(existing *TargetHealth, isHealthy bool, responseTime time.Duration, err error, config config.HealthCheckConfig) *TargetHealth {
	health := &TargetHealth{
		URL:          existing.URL,
		LastCheck:    time.Now(),
		ResponseTime: responseTime,
		Error:        err,
	}

	if isHealthy {
		health.ConsecutiveSuccesses = existing.ConsecutiveSuccesses + 1
		health.ConsecutiveFailures = 0
		
		// Target becomes healthy after consecutive successes
		if health.ConsecutiveSuccesses >= config.SuccessThreshold {
			health.Status = StatusHealthy
		} else {
			health.Status = existing.Status
		}
	} else {
		health.ConsecutiveFailures = existing.ConsecutiveFailures + 1
		health.ConsecutiveSuccesses = 0
		
		// Target becomes unhealthy after consecutive failures
		if health.ConsecutiveFailures >= config.FailureThreshold {
			health.Status = StatusUnhealthy
		} else {
			health.Status = existing.Status
		}
	}

	// Log status changes
	if health.Status != existing.Status {
		if health.Status == StatusHealthy {
			c.logger.Info("Target became healthy",
				zap.String("url", health.URL),
				zap.Int("consecutive_successes", health.ConsecutiveSuccesses))
		} else if health.Status == StatusUnhealthy {
			c.logger.Warn("Target became unhealthy",
				zap.String("url", health.URL),
				zap.Int("consecutive_failures", health.ConsecutiveFailures),
				zap.Error(err))
		}
	}

	return health
}

// IsHealthy returns whether a target is healthy
func (c *checker) IsHealthy(url string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	health, exists := c.targets[url]
	if !exists {
		return true // Default to healthy for unknown targets
	}
	
	return health.Status == StatusHealthy
}

// GetHealth returns the health status of a target
func (c *checker) GetHealth(url string) *TargetHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	health, exists := c.targets[url]
	if !exists {
		return &TargetHealth{
			URL:    url,
			Status: StatusUnknown,
		}
	}
	
	// Return a copy to avoid race conditions
	return &TargetHealth{
		URL:                  health.URL,
		Status:               health.Status,
		LastCheck:            health.LastCheck,
		ConsecutiveFailures:  health.ConsecutiveFailures,
		ConsecutiveSuccesses: health.ConsecutiveSuccesses,
		ResponseTime:         health.ResponseTime,
		Error:                health.Error,
	}
}

// GetAllHealth returns the health status of all targets
func (c *checker) GetAllHealth() map[string]*TargetHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*TargetHealth, len(c.targets))
	for url, health := range c.targets {
		result[url] = &TargetHealth{
			URL:                  health.URL,
			Status:               health.Status,
			LastCheck:            health.LastCheck,
			ConsecutiveFailures:  health.ConsecutiveFailures,
			ConsecutiveSuccesses: health.ConsecutiveSuccesses,
			ResponseTime:         health.ResponseTime,
			Error:                health.Error,
		}
	}
	
	return result
}

// registerTarget registers a target for health monitoring
func (c *checker) registerTarget(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if _, exists := c.targets[url]; !exists {
		c.targets[url] = &TargetHealth{
			URL:    url,
			Status: StatusUnknown,
		}
		c.logger.Debug("Registered target for health monitoring", zap.String("url", url))
	}
}

// unregisterTarget unregisters a target from health monitoring
func (c *checker) unregisterTarget(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.targets, url)
	c.logger.Debug("Unregistered target from health monitoring", zap.String("url", url))
}