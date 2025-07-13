package middleware

import (
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware provides rate limiting functionality
type RateLimitMiddleware struct {
	logger   *zap.Logger
	config   RateLimitConfig
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int    `json:"requests_per_second"`
	Burst             int    `json:"burst"`
	KeyFunc           string `json:"key_func"` // "ip", "user", "global"
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(logger *zap.Logger, config map[string]any) (*RateLimitMiddleware, error) {
	rateLimitConfig := RateLimitConfig{
		RequestsPerSecond: 10.0, // Default: 10 requests per second
		Burst:             20,   // Default: burst of 20
		KeyFunc:           "ip", // Default: rate limit by IP
	}

	if rps, ok := config["requests_per_second"].(int); ok {
		rateLimitConfig.RequestsPerSecond = rps
	}

	if burst, ok := config["burst"].(int); ok {
		rateLimitConfig.Burst = burst
	}

	if keyFunc, ok := config["key_func"].(string); ok {
		rateLimitConfig.KeyFunc = keyFunc
	}

	return &RateLimitMiddleware{
		logger:   logger,
		config:   rateLimitConfig,
		limiters: make(map[string]*rate.Limiter),
	}, nil
}

// Handle implements the middleware interface
func (rlm *RateLimitMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rlm.getKey(r)
		limiter := rlm.getLimiter(key)

		if !limiter.Allow() {
			rlm.logger.Warn("Rate limit exceeded",
				zap.String("key", key),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("path", r.URL.Path))

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.2f", rlm.config.RequestsPerSecond))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", "1")

			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Name returns the middleware name
func (rlm *RateLimitMiddleware) Name() string {
	return "rate_limit"
}

// getKey generates a key for rate limiting based on the configured key function
func (rlm *RateLimitMiddleware) getKey(r *http.Request) string {
	switch rlm.config.KeyFunc {
	case "ip":
		return getClientIP(r)
	case "user":
		// Extract user ID from JWT token or session
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			return userID
		}
		return getClientIP(r) // Fallback to IP
	case "global":
		return "global"
	default:
		return getClientIP(r)
	}
}

// getLimiter gets or creates a rate limiter for the given key
func (rlm *RateLimitMiddleware) getLimiter(key string) *rate.Limiter {
	rlm.mu.RLock()
	limiter, exists := rlm.limiters[key]
	rlm.mu.RUnlock()

	if !exists {
		rlm.mu.Lock()
		// Double-check after acquiring write lock
		if limiter, exists = rlm.limiters[key]; !exists {
			limiter = rate.NewLimiter(rate.Limit(rlm.config.RequestsPerSecond), rlm.config.Burst)
			rlm.limiters[key] = limiter
		}
		rlm.mu.Unlock()
	}

	return limiter
}

// Cleanup removes old limiters (should be called periodically)
func (rlm *RateLimitMiddleware) Cleanup() {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	// Remove limiters that haven't been used recently
	// This is a simple implementation - in production, you might want
	// to use a more sophisticated approach with TTL or LRU cache
	for key, limiter := range rlm.limiters {
		if limiter.Tokens() == float64(rlm.config.Burst) {
			delete(rlm.limiters, key)
		}
	}
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
