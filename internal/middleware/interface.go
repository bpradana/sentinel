package middleware

import (
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"github.com/bpradana/sentinel/internal/config"
	"go.uber.org/zap"
)

// Middleware defines the interface for middleware components
type Middleware interface {
	// Handle processes the request and calls the next handler
	Handle(next http.Handler) http.Handler
	// Name returns the name of the middleware
	Name() string
}

// Chain represents a chain of middleware
type Chain struct {
	middlewares []Middleware
	logger      *zap.Logger
}

// NewChain creates a new middleware chain
func NewChain(logger *zap.Logger) *Chain {
	return &Chain{
		middlewares: make([]Middleware, 0),
		logger:      logger,
	}
}

// Use adds a middleware to the chain
func (c *Chain) Use(middleware Middleware) {
	c.middlewares = append(c.middlewares, middleware)
}

// Then applies the middleware chain to the given handler
func (c *Chain) Then(handler http.Handler) http.Handler {
	// Apply middleware in reverse order so they execute in the correct order
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i].Handle(handler)
	}
	return handler
}

// Factory creates middleware instances
type Factory struct {
	logger *zap.Logger
}

// NewFactory creates a new middleware factory
func NewFactory(logger *zap.Logger) *Factory {
	return &Factory{logger: logger}
}

// CreateChain creates a middleware chain from configuration
func (f *Factory) CreateChain(middlewareConfig *config.MiddlewareConfig) (*Chain, error) {
	chain := NewChain(f.logger)

	// Sort middleware by order
	middlewares := make([]config.MiddlewareChain, len(middlewareConfig.Chain))
	copy(middlewares, middlewareConfig.Chain)
	sort.Slice(middlewares, func(i, j int) bool {
		return middlewares[i].Order < middlewares[j].Order
	})

	// Create and add middleware to chain
	for _, mw := range middlewares {
		if !mw.Enabled {
			continue
		}

		middleware, err := f.Create(mw.Type, mw.Config)
		if err != nil {
			return nil, err
		}

		chain.Use(middleware)
	}

	return chain, nil
}

// Create creates a middleware instance based on type and configuration
func (f *Factory) Create(middlewareType string, config map[string]any) (Middleware, error) {
	switch middlewareType {
	case "logging":
		return NewLoggingMiddleware(f.logger, config)
	case "rate_limit":
		return NewRateLimitMiddleware(f.logger, config)
	case "auth":
		return NewAuthMiddleware(f.logger, config)
	case "compression":
		return NewCompressionMiddleware(f.logger, config)
	default:
		return nil, fmt.Errorf("unknown middleware type: %s", middlewareType)
	}
}

// RequestContext holds request-specific data
type RequestContext struct {
	StartTime time.Time
	RequestID string
	UserAgent string
	ClientIP  string
}

// NewRequestContext creates a new request context
func NewRequestContext() *RequestContext {
	return &RequestContext{
		StartTime: time.Now(),
		RequestID: generateRequestID(),
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(1000))
}
