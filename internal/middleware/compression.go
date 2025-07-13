package middleware

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// CompressionMiddleware handles response compression
type CompressionMiddleware struct {
	logger          *zap.Logger
	level           int
	minLength       int
	compressedTypes []string
	skipPaths       []string
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(logger *zap.Logger, config map[string]any) (*CompressionMiddleware, error) {
	comp := &CompressionMiddleware{
		logger:    logger,
		level:     gzip.DefaultCompression,
		minLength: 1024, // 1KB minimum
		compressedTypes: []string{
			"text/html",
			"text/plain",
			"text/css",
			"text/javascript",
			"application/javascript",
			"application/json",
			"application/xml",
			"text/xml",
		},
	}

	// Parse configuration
	if level, ok := config["level"].(int); ok {
		if level >= gzip.NoCompression && level <= gzip.BestCompression {
			comp.level = level
		}
	}
	if levelFloat, ok := config["level"].(float64); ok {
		level := int(levelFloat)
		if level >= gzip.NoCompression && level <= gzip.BestCompression {
			comp.level = level
		}
	}

	// Support both min_length and min_size parameter names
	if minLength, ok := config["min_length"].(int); ok {
		comp.minLength = minLength
	}
	if minLengthFloat, ok := config["min_length"].(float64); ok {
		comp.minLength = int(minLengthFloat)
	}
	if minSize, ok := config["min_size"].(int); ok {
		comp.minLength = minSize
	}
	if minSizeFloat, ok := config["min_size"].(float64); ok {
		comp.minLength = int(minSizeFloat)
	}

	// Parse compressed types - support both types and content_types parameter names
	if typesInterface, ok := config["types"]; ok {
		if typesSlice, ok := typesInterface.([]any); ok {
			comp.compressedTypes = make([]string, len(typesSlice))
			for i, t := range typesSlice {
				if typeStr, ok := t.(string); ok {
					comp.compressedTypes[i] = typeStr
				}
			}
		} else if typesSlice, ok := typesInterface.([]string); ok {
			comp.compressedTypes = typesSlice
		}
	}
	if contentTypesInterface, ok := config["content_types"]; ok {
		if contentTypesSlice, ok := contentTypesInterface.([]any); ok {
			comp.compressedTypes = make([]string, len(contentTypesSlice))
			for i, t := range contentTypesSlice {
				if typeStr, ok := t.(string); ok {
					comp.compressedTypes[i] = typeStr
				}
			}
		} else if contentTypesSlice, ok := contentTypesInterface.([]string); ok {
			comp.compressedTypes = contentTypesSlice
		}
	}

	// Parse skip paths
	if skipPathsInterface, ok := config["skip_paths"]; ok {
		if skipPathsSlice, ok := skipPathsInterface.([]any); ok {
			comp.skipPaths = make([]string, len(skipPathsSlice))
			for i, path := range skipPathsSlice {
				if pathStr, ok := path.(string); ok {
					comp.skipPaths[i] = pathStr
				}
			}
		} else if skipPathsSlice, ok := skipPathsInterface.([]string); ok {
			comp.skipPaths = skipPathsSlice
		}
	}

	return comp, nil
}

// Handle processes the request with compression
func (c *CompressionMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should be skipped
		for _, skipPath := range c.skipPaths {
			if strings.HasPrefix(r.URL.Path, skipPath) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Create compressed response writer
		cw := &compressedResponseWriter{
			ResponseWriter: w,
			middleware:     c,
			request:        r,
		}

		// Serve the request
		next.ServeHTTP(cw, r)

		// Close the gzip writer if it was created
		if cw.gzipWriter != nil {
			cw.gzipWriter.Close()
		}
	})
}

// Name returns the middleware name
func (c *CompressionMiddleware) Name() string {
	return "compression"
}

// shouldCompress determines if the response should be compressed
func (c *CompressionMiddleware) shouldCompress(contentType string, contentLength int) bool {
	// Check minimum length
	if contentLength > 0 && contentLength < c.minLength {
		return false
	}

	// Check if content type should be compressed
	for _, compressedType := range c.compressedTypes {
		if strings.Contains(contentType, compressedType) {
			return true
		}
	}

	return false
}

// compressedResponseWriter wraps http.ResponseWriter to provide compression
type compressedResponseWriter struct {
	http.ResponseWriter
	middleware  *CompressionMiddleware
	request     *http.Request
	gzipWriter  *gzip.Writer
	wroteHeader bool
}

// WriteHeader captures the status code and headers
func (cw *compressedResponseWriter) WriteHeader(statusCode int) {
	if cw.wroteHeader {
		return
	}
	cw.wroteHeader = true

	// Don't compress error responses
	if statusCode >= 400 {
		cw.ResponseWriter.WriteHeader(statusCode)
		return
	}

	// Check if we should compress based on content type
	contentType := cw.Header().Get("Content-Type")
	contentLength := 0
	if cl := cw.Header().Get("Content-Length"); cl != "" {
		fmt.Sscanf(cl, "%d", &contentLength)
	}

	if cw.middleware.shouldCompress(contentType, contentLength) {
		// Set compression headers
		cw.Header().Set("Content-Encoding", "gzip")
		cw.Header().Set("Vary", "Accept-Encoding")
		cw.Header().Del("Content-Length") // Remove content-length as it will change

		// Create gzip writer
		var err error
		cw.gzipWriter, err = gzip.NewWriterLevel(cw.ResponseWriter, cw.middleware.level)
		if err != nil {
			cw.middleware.logger.Error("Failed to create gzip writer", zap.Error(err))
			cw.ResponseWriter.WriteHeader(statusCode)
			return
		}

		cw.middleware.logger.Debug("Compressing response",
			zap.String("path", cw.request.URL.Path),
			zap.String("content-type", contentType),
			zap.Int("content-length", contentLength),
		)
	}

	cw.ResponseWriter.WriteHeader(statusCode)
}

// Write writes data to the response
func (cw *compressedResponseWriter) Write(data []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	if cw.gzipWriter != nil {
		return cw.gzipWriter.Write(data)
	}

	return cw.ResponseWriter.Write(data)
}

// Flush flushes the response
func (cw *compressedResponseWriter) Flush() {
	if cw.gzipWriter != nil {
		cw.gzipWriter.Flush()
	}
	if flusher, ok := cw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
