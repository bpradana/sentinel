package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// LoggingMiddleware provides structured request logging
type LoggingMiddleware struct {
	logger *zap.Logger
	config LoggingConfig
}

// LoggingConfig holds logging middleware configuration
type LoggingConfig struct {
	LogHeaders   bool `json:"log_headers"`
	LogBody      bool `json:"log_body"`
	LogRequests  bool `json:"log_requests"`
	LogResponses bool `json:"log_responses"`
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger *zap.Logger, config map[string]any) (*LoggingMiddleware, error) {
	loggingConfig := LoggingConfig{
		LogRequests:  true,  // Default to true
		LogResponses: true,  // Default to true
		LogHeaders:   false, // Default to false
		LogBody:      false, // Default to false
	}

	if logHeaders, ok := config["log_headers"].(bool); ok {
		loggingConfig.LogHeaders = logHeaders
	}

	if logBody, ok := config["log_body"].(bool); ok {
		loggingConfig.LogBody = logBody
	}

	if logRequests, ok := config["log_requests"].(bool); ok {
		loggingConfig.LogRequests = logRequests
	}

	if logResponses, ok := config["log_responses"].(bool); ok {
		loggingConfig.LogResponses = logResponses
	}

	return &LoggingMiddleware{
		logger: logger,
		config: loggingConfig,
	}, nil
}

// Handle implements the middleware interface
func (lm *LoggingMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures status code and size
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
			size:           0,
		}

		// Log request if enabled
		if lm.config.LogRequests {
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("proto", r.Proto),
				zap.String("host", r.Host),
			}

			if lm.config.LogHeaders {
				for name, values := range r.Header {
					for _, value := range values {
						fields = append(fields, zap.String("header_"+name, value))
					}
				}
			}

			lm.logger.Info("Request started", fields...)
		}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Log response if enabled
		if lm.config.LogResponses {
			duration := time.Since(start)
			responseFields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.statusCode),
				zap.Int64("size", rw.size),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
			}

			if rw.statusCode >= 400 {
				lm.logger.Error("Request completed with error", responseFields...)
			} else {
				lm.logger.Info("Request completed", responseFields...)
			}
		}
	})
}

// Name returns the middleware name
func (lm *LoggingMiddleware) Name() string {
	return "logging"
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size
func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += int64(size)
	return size, err
}
