package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bpradana/sentinel/internal/config"
	"go.uber.org/zap"
)

// Server handles metrics collection and serving
type Server struct {
	cfg    *config.MetricsConfig
	logger *zap.Logger
	server *http.Server
}

// NewServer creates a new metrics server
func NewServer(cfg *config.MetricsConfig, logger *zap.Logger) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
	}
}

// Start starts the metrics server
func (s *Server) Start() error {
	if !s.cfg.Enabled {
		s.logger.Info("Metrics server disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Path, s.metricsHandler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	s.logger.Info("Starting metrics server",
		zap.Int("port", s.cfg.Port),
		zap.String("path", s.cfg.Path))

	return s.server.ListenAndServe()
}

// Stop stops the metrics server
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping metrics server")
	return s.server.Close()
}

// metricsHandler handles metrics requests
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Basic metrics for now - this can be expanded with actual metrics collection
	metrics := `# HELP sentinel_requests_total Total number of requests
# TYPE sentinel_requests_total counter
sentinel_requests_total 0

# HELP sentinel_requests_duration_seconds Request duration in seconds
# TYPE sentinel_requests_duration_seconds histogram
sentinel_requests_duration_seconds 0

# HELP sentinel_upstream_health_up Upstream health status
# TYPE sentinel_upstream_health_up gauge
sentinel_upstream_health_up 1

# HELP sentinel_tls_certificates_total Total number of TLS certificates
# TYPE sentinel_tls_certificates_total gauge
sentinel_tls_certificates_total 0
`

	w.Write([]byte(metrics))
}
