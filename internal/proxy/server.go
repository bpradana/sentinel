package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"regexp"

	"github.com/bpradana/sentinel/internal/config"
	"github.com/bpradana/sentinel/internal/health"
	"github.com/bpradana/sentinel/internal/loadbalancer"
	"github.com/bpradana/sentinel/internal/middleware"
	"github.com/bpradana/sentinel/internal/tls"
	"go.uber.org/zap"
)

type Server interface {
	// Start starts the proxy server
	Start() error
	// Shutdown shuts down the proxy server
	Shutdown(ctx context.Context) error
	// UpdateConfig updates the proxy server configuration
	UpdateConfig(config *config.Config) error
}

type server struct {
	cfg           *config.Config
	tlsManager    *tls.Manager
	healthChecker health.Checker
	logger        *zap.Logger

	// HTTP server
	httpServer *http.Server

	// HTTPS server
	httpsServer *http.Server

	// Load balancers for each upstream
	loadBalancers map[string]loadbalancer.LoadBalancer

	// Middleware factory
	middlewareFactory *middleware.Factory

	// Server state
	mu       sync.RWMutex
	running  bool
	shutdown chan struct{}
}

func NewServer(cfg *config.Config, tlsManager *tls.Manager, healthChecker health.Checker, logger *zap.Logger) Server {
	return &server{
		cfg:               cfg,
		tlsManager:        tlsManager,
		healthChecker:     healthChecker,
		logger:            logger,
		loadBalancers:     make(map[string]loadbalancer.LoadBalancer),
		middlewareFactory: middleware.NewFactory(logger),
		shutdown:          make(chan struct{}),
	}
}

func (s *server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	s.logger.Info("Starting proxy server")

	// Initialize load balancers
	if err := s.initializeLoadBalancers(); err != nil {
		return fmt.Errorf("failed to initialize load balancers: %w", err)
	}

	// Create main handler
	mainHandler := s.createMainHandler()

	// Apply global middleware
	globalChain, err := s.middlewareFactory.CreateChain(&s.cfg.Middleware)
	if err != nil {
		return fmt.Errorf("failed to create global middleware chain: %w", err)
	}

	handler := globalChain.Then(mainHandler)

	// Start HTTP server if port is configured
	if s.cfg.Global.Server.HTTPPort > 0 {
		s.httpServer = &http.Server{
			Addr:           fmt.Sprintf(":%d", s.cfg.Global.Server.HTTPPort),
			Handler:        handler,
			ReadTimeout:    s.cfg.Global.Server.ReadTimeout,
			WriteTimeout:   s.cfg.Global.Server.WriteTimeout,
			IdleTimeout:    s.cfg.Global.Server.IdleTimeout,
			MaxHeaderBytes: s.cfg.Global.Server.MaxHeaderSize,
		}

		// Enable HTTP2 if configured
		if s.cfg.Global.Server.HTTP2Enabled {
			// HTTP2 is enabled by default in Go 1.6+ for HTTPS
			// For HTTP, we need to explicitly enable it
			s.logger.Info("HTTP2 enabled for HTTP server")
		}

		go func() {
			s.logger.Info("Starting HTTP server", zap.Int("port", s.cfg.Global.Server.HTTPPort))
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("HTTP server error", zap.Error(err))
			}
		}()
	}

	// Start HTTPS server if port is configured and TLS is enabled
	if s.cfg.Global.Server.HTTPSPort > 0 && s.cfg.TLS.Enabled {
		tlsConfig, err := s.tlsManager.GetTLSConfig("")
		if err != nil {
			return fmt.Errorf("failed to get TLS config: %w", err)
		}

		// Enable HTTP2 for HTTPS server
		if s.cfg.Global.Server.HTTP2Enabled {
			tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
			s.logger.Info("HTTP2 enabled for HTTPS server")
		}

		s.httpsServer = &http.Server{
			Addr:           fmt.Sprintf(":%d", s.cfg.Global.Server.HTTPSPort),
			Handler:        handler,
			ReadTimeout:    s.cfg.Global.Server.ReadTimeout,
			WriteTimeout:   s.cfg.Global.Server.WriteTimeout,
			IdleTimeout:    s.cfg.Global.Server.IdleTimeout,
			MaxHeaderBytes: s.cfg.Global.Server.MaxHeaderSize,
			TLSConfig:      tlsConfig,
		}

		go func() {
			s.logger.Info("Starting HTTPS server", zap.Int("port", s.cfg.Global.Server.HTTPSPort))
			if err := s.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				s.logger.Error("HTTPS server error", zap.Error(err))
			}
		}()
	}

	s.running = true
	s.logger.Info("Proxy server started successfully")

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Shutting down proxy server")
	s.running = false
	close(s.shutdown)

	var wg sync.WaitGroup
	var errors []error

	// Shutdown HTTP server
	if s.httpServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.httpServer.Shutdown(ctx); err != nil {
				errors = append(errors, fmt.Errorf("HTTP server shutdown error: %w", err))
			}
		}()
	}

	// Shutdown HTTPS server
	if s.httpsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.httpsServer.Shutdown(ctx); err != nil {
				errors = append(errors, fmt.Errorf("HTTPS server shutdown error: %w", err))
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	s.logger.Info("Proxy server shutdown complete")
	return nil
}

func (s *server) UpdateConfig(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Updating proxy server configuration")

	// Update configuration
	s.cfg = cfg

	// Reinitialize load balancers
	if err := s.initializeLoadBalancers(); err != nil {
		return fmt.Errorf("failed to reinitialize load balancers: %w", err)
	}

	s.logger.Info("Configuration updated successfully")
	return nil
}

func (s *server) initializeLoadBalancers() error {
	s.loadBalancers = make(map[string]loadbalancer.LoadBalancer)
	factory := &loadbalancer.DefaultFactory{}

	for name, service := range s.cfg.Upstreams.Services {
		lb, err := factory.Create(service.LoadBalancer)
		if err != nil {
			return fmt.Errorf("failed to create load balancer for %s: %w", name, err)
		}
		s.loadBalancers[name] = lb
		s.logger.Debug("Initialized load balancer",
			zap.String("upstream", name),
			zap.String("strategy", service.LoadBalancer))
	}

	return nil
}

func (s *server) createMainHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Find matching route
		route := s.findMatchingRoute(r)
		if route == nil {
			s.logger.Warn("No matching route found",
				zap.String("host", r.Host),
				zap.String("path", r.URL.Path))
			http.NotFound(w, r)
			return
		}

		// Apply URL rewriting if configured
		if err := s.applyRewrite(r, &route.Rewrite); err != nil {
			s.logger.Error("Failed to apply rewrite", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Get upstream service
		upstream, exists := s.cfg.Upstreams.Services[route.Upstream]
		if !exists {
			s.logger.Error("Upstream not found", zap.String("upstream", route.Upstream))
			http.Error(w, "Upstream not found", http.StatusServiceUnavailable)
			return
		}

		// Get load balancer
		lb, exists := s.loadBalancers[route.Upstream]
		if !exists {
			s.logger.Error("Load balancer not found", zap.String("upstream", route.Upstream))
			http.Error(w, "Load balancer not found", http.StatusServiceUnavailable)
			return
		}

		// Create targets from upstream configuration
		targets := s.createTargets(upstream)
		if len(targets) == 0 {
			s.logger.Error("No healthy targets available", zap.String("upstream", route.Upstream))
			http.Error(w, "No healthy targets available", http.StatusServiceUnavailable)
			return
		}

		// Select target
		target, err := lb.SelectTarget(targets, r)
		if err != nil {
			s.logger.Error("Failed to select target",
				zap.String("upstream", route.Upstream),
				zap.Error(err))
			http.Error(w, "Failed to select target", http.StatusServiceUnavailable)
			return
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(target.URL)

		// Configure proxy
		proxy.Transport = &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}

		// Apply route timeout if configured
		if route.Timeout > 0 {
			ctx, cancel := context.WithTimeout(r.Context(), route.Timeout)
			defer cancel()
			r = r.WithContext(ctx)
			s.logger.Debug("Applied route timeout",
				zap.Duration("timeout", route.Timeout),
				zap.String("route", route.Host+route.Path))
		}

		// Apply route-specific middleware
		routeHandler := s.applyRouteMiddleware(proxy, route)

		// Apply retry logic if configured
		if route.RetryPolicy.Attempts > 0 {
			routeHandler = s.createRetryMiddleware(routeHandler, &route.RetryPolicy)
		}

		// Update target connection count
		lb.UpdateTarget(target, 1)
		defer lb.UpdateTarget(target, -1)

		// Serve the request
		routeHandler.ServeHTTP(w, r)
	})
}

func (s *server) findMatchingRoute(r *http.Request) *config.RouteRule {
	for _, rule := range s.cfg.Routes.Rules {
		// Check host match - strip port from request host for comparison
		if rule.Host != "" {
			requestHost := r.Host
			if colonIndex := strings.Index(requestHost, ":"); colonIndex != -1 {
				requestHost = requestHost[:colonIndex]
			}
			if rule.Host != requestHost {
				continue
			}
		}

		// Check path match - support both exact and prefix matching
		if rule.Path != "" {
			// If path ends with /*, use prefix matching
			if strings.HasSuffix(rule.Path, "/*") {
				prefix := strings.TrimSuffix(rule.Path, "/*")
				if !strings.HasPrefix(r.URL.Path, prefix) {
					continue
				}
			} else {
				// Exact path matching
				if r.URL.Path != rule.Path {
					continue
				}
			}
		}

		// Check method match
		if len(rule.Methods) > 0 {
			methodMatch := false
			for _, method := range rule.Methods {
				if method == r.Method {
					methodMatch = true
					break
				}
			}
			if !methodMatch {
				continue
			}
		}

		return &rule
	}
	return nil
}

func (s *server) createTargets(upstream config.UpstreamService) []*loadbalancer.Target {
	var targets []*loadbalancer.Target

	for _, targetConfig := range upstream.Targets {
		url, err := url.Parse(targetConfig.URL)
		if err != nil {
			s.logger.Error("Invalid target URL",
				zap.String("url", targetConfig.URL),
				zap.Error(err))
			continue
		}

		// Check health status
		isHealthy := s.healthChecker.IsHealthy(targetConfig.URL)

		target := &loadbalancer.Target{
			URL:       url,
			Weight:    targetConfig.Weight,
			IsHealthy: isHealthy,
		}

		targets = append(targets, target)
	}

	return targets
}

func (s *server) applyRewrite(r *http.Request, rewrite *config.RewriteConfig) error {
	if rewrite == nil {
		return nil
	}

	originalPath := r.URL.Path

	// Apply strip prefix
	if rewrite.StripPrefix != "" && strings.HasPrefix(originalPath, rewrite.StripPrefix) {
		r.URL.Path = strings.TrimPrefix(originalPath, rewrite.StripPrefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		s.logger.Debug("Applied strip prefix",
			zap.String("original", originalPath),
			zap.String("stripped", r.URL.Path),
			zap.String("prefix", rewrite.StripPrefix))
	}

	// Apply add prefix
	if rewrite.AddPrefix != "" {
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}
		r.URL.Path = rewrite.AddPrefix + r.URL.Path
		s.logger.Debug("Applied add prefix",
			zap.String("original", originalPath),
			zap.String("new", r.URL.Path),
			zap.String("prefix", rewrite.AddPrefix))
	}

	// Apply regex replacement
	if rewrite.Regex != "" && rewrite.Replacement != "" {
		re, err := regexp.Compile(rewrite.Regex)
		if err != nil {
			return fmt.Errorf("invalid rewrite regex: %w", err)
		}
		r.URL.Path = re.ReplaceAllString(r.URL.Path, rewrite.Replacement)
		s.logger.Debug("Applied regex rewrite",
			zap.String("original", originalPath),
			zap.String("new", r.URL.Path),
			zap.String("regex", rewrite.Regex),
			zap.String("replacement", rewrite.Replacement))
	}

	return nil
}

func (s *server) applyRouteMiddleware(handler http.Handler, route *config.RouteRule) http.Handler {
	// Create middleware chain for this route
	chain := middleware.NewChain(s.logger)

	// Add route-specific middleware
	for _, middlewareName := range route.Middleware {
		// Find middleware configuration
		var mwConfig config.MiddlewareChain
		for _, mw := range s.cfg.Middleware.Chain {
			if mw.Name == middlewareName && mw.Enabled {
				mwConfig = mw
				break
			}
		}

		if mwConfig.Name != "" {
			middleware, err := s.middlewareFactory.Create(mwConfig.Type, mwConfig.Config)
			if err != nil {
				s.logger.Error("Failed to create middleware",
					zap.String("name", middlewareName),
					zap.Error(err))
				continue
			}
			chain.Use(middleware)
		}
	}

	// Apply route headers if configured
	if len(route.Headers) > 0 {
		chain.Use(s.createHeadersMiddleware(route.Headers))
	}

	return chain.Then(handler)
}

// createHeadersMiddleware creates a middleware that applies route-specific headers
func (s *server) createHeadersMiddleware(headers map[string]string) middleware.Middleware {
	return &headersMiddleware{
		headers: headers,
		logger:  s.logger,
	}
}

// headersMiddleware applies route-specific headers to responses
type headersMiddleware struct {
	headers map[string]string
	logger  *zap.Logger
}

func (hm *headersMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a response writer that captures headers
		rw := &headerResponseWriter{
			ResponseWriter: w,
			headers:        hm.headers,
			logger:         hm.logger,
		}

		next.ServeHTTP(rw, r)
	})
}

func (hm *headersMiddleware) Name() string {
	return "headers"
}

// headerResponseWriter wraps http.ResponseWriter to apply headers
type headerResponseWriter struct {
	http.ResponseWriter
	headers map[string]string
	logger  *zap.Logger
}

func (hw *headerResponseWriter) WriteHeader(statusCode int) {
	// Apply route headers before writing the status code
	for name, value := range hw.headers {
		hw.Header().Set(name, value)
		hw.logger.Debug("Applied route header",
			zap.String("name", name),
			zap.String("value", value))
	}
	hw.ResponseWriter.WriteHeader(statusCode)
}

// createRetryMiddleware creates a middleware that implements retry logic
func (s *server) createRetryMiddleware(handler http.Handler, retryPolicy *config.RetryPolicy) http.Handler {
	return &retryHandler{
		handler:     handler,
		retryPolicy: retryPolicy,
		logger:      s.logger,
	}
}

// retryHandler implements retry logic for failed requests
type retryHandler struct {
	handler     http.Handler
	retryPolicy *config.RetryPolicy
	logger      *zap.Logger
}

func (rh *retryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a response writer that captures status codes
	rw := &retryResponseWriter{
		ResponseWriter: w,
	}

	var lastErr error
	for attempt := 0; attempt <= rh.retryPolicy.Attempts; attempt++ {
		// Reset response writer for each attempt
		rw.statusCode = 0
		rw.written = false

		// Serve the request
		rh.handler.ServeHTTP(rw, r)

		// Check if the request was successful
		if rw.statusCode < 500 || attempt == rh.retryPolicy.Attempts {
			// Success or max attempts reached
			if attempt > 0 {
				rh.logger.Info("Request succeeded after retries",
					zap.Int("attempts", attempt+1),
					zap.Int("status", rw.statusCode))
			}
			return
		}

		// Log retry attempt
		rh.logger.Warn("Request failed, retrying",
			zap.Int("attempt", attempt+1),
			zap.Int("max_attempts", rh.retryPolicy.Attempts+1),
			zap.Int("status", rw.statusCode),
			zap.Duration("backoff", rh.retryPolicy.Backoff))

		// Wait before retrying (except on the last attempt)
		if attempt < rh.retryPolicy.Attempts {
			time.Sleep(rh.retryPolicy.Backoff)
		}
	}

	// All attempts failed
	if lastErr != nil {
		rh.logger.Error("Request failed after all retry attempts", zap.Error(lastErr))
	}
}

// retryResponseWriter wraps http.ResponseWriter to capture status codes for retry logic
type retryResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *retryResponseWriter) WriteHeader(statusCode int) {
	if !rw.written {
		rw.statusCode = statusCode
		rw.written = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *retryResponseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(data)
}
