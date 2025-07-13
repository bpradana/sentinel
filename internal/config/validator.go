package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// ValidateConfig validates the entire configuration
func ValidateConfig(config *Config, log *zap.Logger) error {
	if err := validateGlobalConfig(&config.Global, log); err != nil {
		log.Error("Global config validation failed", zap.Error(err))
		return fmt.Errorf("global config validation failed: %w", err)
	}

	if err := validateUpstreamsConfig(&config.Upstreams, log); err != nil {
		log.Error("Upstreams config validation failed", zap.Error(err))
		return fmt.Errorf("upstreams config validation failed: %w", err)
	}

	if err := validateRoutesConfig(&config.Routes, &config.Upstreams, log); err != nil {
		log.Error("Routes config validation failed", zap.Error(err))
		return fmt.Errorf("routes config validation failed: %w", err)
	}

	if err := validateMiddlewareConfig(&config.Middleware, log); err != nil {
		log.Error("Middleware config validation failed", zap.Error(err))
		return fmt.Errorf("middleware config validation failed: %w", err)
	}

	if err := validateTLSConfig(&config.TLS, log); err != nil {
		log.Error("TLS config validation failed", zap.Error(err))
		return fmt.Errorf("TLS config validation failed: %w", err)
	}

	return nil
}

// validateGlobalConfig validates global configuration
func validateGlobalConfig(config *GlobalConfig, log *zap.Logger) error {
	if config.Server.HTTPPort < 1 || config.Server.HTTPPort > 65535 {
		log.Error("Invalid HTTP port", zap.Int("port", config.Server.HTTPPort))
		return fmt.Errorf("invalid HTTP port: %d", config.Server.HTTPPort)
	}

	if config.Server.HTTPSPort < 1 || config.Server.HTTPSPort > 65535 {
		log.Error("Invalid HTTPS port", zap.Int("port", config.Server.HTTPSPort))
		return fmt.Errorf("invalid HTTPS port: %d", config.Server.HTTPSPort)
	}

	if config.Server.HTTPPort == config.Server.HTTPSPort {
		log.Error("HTTP and HTTPS ports cannot be the same", zap.Int("http_port", config.Server.HTTPPort), zap.Int("https_port", config.Server.HTTPSPort))
		return fmt.Errorf("HTTP and HTTPS ports cannot be the same")
	}

	if config.Server.ReadTimeout < 0 {
		log.Error("Read timeout cannot be negative", zap.Duration("timeout", config.Server.ReadTimeout))
		return fmt.Errorf("read timeout cannot be negative")
	}

	if config.Server.WriteTimeout < 0 {
		log.Error("Write timeout cannot be negative", zap.Duration("timeout", config.Server.WriteTimeout))
		return fmt.Errorf("write timeout cannot be negative")
	}

	if config.Server.IdleTimeout < 0 {
		log.Error("Idle timeout cannot be negative", zap.Duration("timeout", config.Server.IdleTimeout))
		return fmt.Errorf("idle timeout cannot be negative")
	}

	if config.Server.MaxHeaderSize < 1024 {
		log.Error("Max header size must be at least 1024 bytes", zap.Int("size", config.Server.MaxHeaderSize))
		return fmt.Errorf("max header size must be at least 1024 bytes")
	}

	// HTTP2Enabled is a boolean, no validation needed

	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, config.Log.Level) {
		log.Error("Invalid log level", zap.String("level", config.Log.Level))
		return fmt.Errorf("invalid log level: %s, must be one of: %s",
			config.Log.Level, strings.Join(validLogLevels, ", "))
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, config.Log.Format) {
		log.Error("Invalid log format", zap.String("format", config.Log.Format))
		return fmt.Errorf("invalid log format: %s, must be one of: %s",
			config.Log.Format, strings.Join(validLogFormats, ", "))
	}

	return nil
}

// validateUpstreamsConfig validates upstream configurations
func validateUpstreamsConfig(config *UpstreamsConfig, log *zap.Logger) error {
	if len(config.Services) == 0 {
		log.Error("At least one upstream service must be defined")
		return fmt.Errorf("at least one upstream service must be defined")
	}

	for name, service := range config.Services {
		if err := validateUpstreamService(name, &service, log); err != nil {
			log.Error("Upstream service validation failed", zap.String("service", name), zap.Error(err))
			return fmt.Errorf("upstream service '%s' validation failed: %w", name, err)
		}
	}

	return nil
}

// validateUpstreamService validates a single upstream service
func validateUpstreamService(name string, service *UpstreamService, log *zap.Logger) error {
	if name == "" {
		log.Error("Upstream service name cannot be empty")
		return fmt.Errorf("upstream service name cannot be empty")
	}

	validLBStrategies := []string{"round_robin", "least_connections", "ip_hash"}
	if !contains(validLBStrategies, service.LoadBalancer) {
		log.Error("Invalid load balancer strategy", zap.String("strategy", service.LoadBalancer))
		return fmt.Errorf("invalid load balancer strategy: %s, must be one of: %s",
			service.LoadBalancer, strings.Join(validLBStrategies, ", "))
	}

	if len(service.Targets) == 0 {
		log.Error("At least one target must be defined")
		return fmt.Errorf("at least one target must be defined")
	}

	for i, target := range service.Targets {
		if err := validateTarget(&target, log); err != nil {
			log.Error("Target validation failed", zap.Int("target", i), zap.Error(err))
			return fmt.Errorf("target %d validation failed: %w", i, err)
		}
	}

	if service.HealthCheck.Enabled {
		if err := validateHealthCheck(&service.HealthCheck, log); err != nil {
			log.Error("Health check validation failed", zap.Error(err))
			return fmt.Errorf("health check validation failed: %w", err)
		}
	}

	return nil
}

// validateTarget validates an upstream target
func validateTarget(target *Target, log *zap.Logger) error {
	if target.URL == "" {
		log.Error("Target URL cannot be empty")
		return fmt.Errorf("target URL cannot be empty")
	}

	parsedURL, err := url.Parse(target.URL)
	if err != nil {
		log.Error("Invalid target URL", zap.String("url", target.URL), zap.Error(err))
		return fmt.Errorf("invalid target URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Error("Target URL scheme must be http or https")
		return fmt.Errorf("target URL scheme must be http or https")
	}

	if parsedURL.Host == "" {
		log.Error("Target URL must have a host")
		return fmt.Errorf("target URL must have a host")
	}

	if target.Weight < 0 {
		log.Error("Target weight cannot be negative")
		return fmt.Errorf("target weight cannot be negative")
	}

	return nil
}

// validateHealthCheck validates health check configuration
func validateHealthCheck(hc *HealthCheckConfig, log *zap.Logger) error {
	if hc.Path == "" {
		log.Error("Health check path cannot be empty")
		return fmt.Errorf("health check path cannot be empty")
	}

	if !strings.HasPrefix(hc.Path, "/") {
		log.Error("Health check path must start with '/'")
		return fmt.Errorf("health check path must start with '/'")
	}

	if hc.Interval <= 0 {
		log.Error("Health check interval must be positive")
		return fmt.Errorf("health check interval must be positive")
	}

	if hc.Timeout <= 0 {
		log.Error("Health check timeout must be positive")
		return fmt.Errorf("health check timeout must be positive")
	}

	if hc.FailureThreshold <= 0 {
		log.Error("Health check failure threshold must be positive")
		return fmt.Errorf("health check failure threshold must be positive")
	}

	if hc.SuccessThreshold <= 0 {
		log.Error("Health check success threshold must be positive")
		return fmt.Errorf("health check success threshold must be positive")
	}

	return nil
}

// validateRoutesConfig validates route configurations
func validateRoutesConfig(config *RoutesConfig, upstreams *UpstreamsConfig, log *zap.Logger) error {
	if len(config.Rules) == 0 {
		log.Error("At least one route rule must be defined")
		return fmt.Errorf("at least one route rule must be defined")
	}

	for i, rule := range config.Rules {
		if err := validateRouteRule(&rule, upstreams, log); err != nil {
			log.Error("Route rule validation failed", zap.Int("rule", i), zap.Error(err))
			return fmt.Errorf("route rule %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validateRouteRule validates a single route rule
func validateRouteRule(rule *RouteRule, upstreams *UpstreamsConfig, log *zap.Logger) error {
	if rule.Host == "" {
		log.Error("Route host cannot be empty")
		return fmt.Errorf("route host cannot be empty")
	}

	if rule.Path == "" {
		log.Error("Route path cannot be empty")
		return fmt.Errorf("route path cannot be empty")
	}

	if !strings.HasPrefix(rule.Path, "/") {
		log.Error("Route path must start with '/'")
		return fmt.Errorf("route path must start with '/'")
	}

	if rule.Upstream == "" {
		log.Error("Route upstream cannot be empty")
		return fmt.Errorf("route upstream cannot be empty")
	}

	if _, exists := upstreams.Services[rule.Upstream]; !exists {
		log.Error("Upstream service not found", zap.String("upstream", rule.Upstream))
		return fmt.Errorf("upstream service '%s' not found", rule.Upstream)
	}

	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range rule.Methods {
		if !contains(validMethods, method) {
			log.Error("Invalid HTTP method", zap.String("method", method))
			return fmt.Errorf("invalid HTTP method: %s", method)
		}
	}

	if rule.Rewrite.Regex != "" {
		if _, err := regexp.Compile(rule.Rewrite.Regex); err != nil {
			log.Error("Invalid rewrite regex", zap.String("regex", rule.Rewrite.Regex), zap.Error(err))
			return fmt.Errorf("invalid rewrite regex: %w", err)
		}
	}

	// Validate rewrite configuration
	if rule.Rewrite.StripPrefix != "" && !strings.HasPrefix(rule.Rewrite.StripPrefix, "/") {
		log.Error("Rewrite strip_prefix must start with '/'")
		return fmt.Errorf("rewrite strip_prefix must start with '/'")
	}

	if rule.Rewrite.AddPrefix != "" && !strings.HasPrefix(rule.Rewrite.AddPrefix, "/") {
		log.Error("Rewrite add_prefix must start with '/'")
		return fmt.Errorf("rewrite add_prefix must start with '/'")
	}

	if rule.Rewrite.Regex != "" && rule.Rewrite.Replacement == "" {
		log.Error("Rewrite replacement is required when regex is specified")
		return fmt.Errorf("rewrite replacement is required when regex is specified")
	}

	if rule.Timeout < 0 {
		log.Error("Route timeout cannot be negative")
		return fmt.Errorf("route timeout cannot be negative")
	}

	if rule.RetryPolicy.Attempts < 0 {
		log.Error("Retry attempts cannot be negative")
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if rule.RetryPolicy.Backoff < 0 {
		log.Error("Retry backoff cannot be negative")
		return fmt.Errorf("retry backoff cannot be negative")
	}

	return nil
}

// validateMiddlewareConfig validates middleware configuration
func validateMiddlewareConfig(config *MiddlewareConfig, log *zap.Logger) error {
	orders := make(map[int]bool)
	names := make(map[string]bool)

	for i, middleware := range config.Chain {
		if middleware.Name == "" {
			log.Error("Middleware name cannot be empty", zap.Int("middleware", i))
			return fmt.Errorf("middleware %d name cannot be empty", i)
		}

		if names[middleware.Name] {
			log.Error("Duplicate middleware name", zap.String("name", middleware.Name))
			return fmt.Errorf("duplicate middleware name: %s", middleware.Name)
		}
		names[middleware.Name] = true

		if orders[middleware.Order] {
			log.Error("Duplicate middleware order", zap.Int("order", middleware.Order))
			return fmt.Errorf("duplicate middleware order: %d", middleware.Order)
		}
		orders[middleware.Order] = true

		validTypes := []string{"logging", "rate_limit", "auth", "cors", "compression"}
		if !contains(validTypes, middleware.Type) {
			log.Error("Invalid middleware type", zap.String("type", middleware.Type))
			return fmt.Errorf("invalid middleware type: %s, must be one of: %s",
				middleware.Type, strings.Join(validTypes, ", "))
		}

		// Validate middleware-specific configuration
		if err := validateMiddlewareSpecificConfig(middleware.Type, middleware.Config, log); err != nil {
			log.Error("Middleware config validation failed", zap.String("middleware", middleware.Name), zap.Error(err))
			return fmt.Errorf("middleware '%s' config validation failed: %w", middleware.Name, err)
		}
	}

	return nil
}

// validateMiddlewareSpecificConfig validates middleware-specific configuration
func validateMiddlewareSpecificConfig(middlewareType string, config map[string]any, log *zap.Logger) error {
	switch middlewareType {
	case "auth":
		// Validate auth middleware config
		if secret, ok := config["jwt_secret"].(string); !ok || secret == "" {
			if secretKey, ok := config["secret_key"].(string); !ok || secretKey == "" {
				log.Error("Auth middleware requires jwt_secret or secret_key")
				return fmt.Errorf("auth middleware requires jwt_secret or secret_key")
			}
		}
	case "rate_limit":
		// Validate rate limit middleware config
		if rps, ok := config["requests_per_second"].(int); !ok || rps <= 0 {
			log.Error("Rate limit middleware requires positive requests_per_second")
			return fmt.Errorf("rate_limit middleware requires positive requests_per_second")
		}
		if burst, ok := config["burst"].(int); !ok || burst <= 0 {
			log.Error("Rate limit middleware requires positive burst")
			return fmt.Errorf("rate_limit middleware requires positive burst")
		}
		if keyFunc, ok := config["key_func"].(string); ok {
			validKeyFuncs := []string{"ip", "user", "global"}
			if !contains(validKeyFuncs, keyFunc) {
				log.Error("Invalid key_func", zap.String("key_func", keyFunc))
				return fmt.Errorf("invalid key_func: %s, must be one of: %s",
					keyFunc, strings.Join(validKeyFuncs, ", "))
			}
		}
	case "compression":
		// Validate compression middleware config
		if level, ok := config["level"].(float64); ok {
			if level < 0 || level > 9 {
				log.Error("Compression level must be between 0 and 9")
				return fmt.Errorf("compression level must be between 0 and 9")
			}
		}
		if minSize, ok := config["min_size"].(float64); ok {
			if minSize < 0 {
				log.Error("Compression min_size cannot be negative")
				return fmt.Errorf("compression min_size cannot be negative")
			}
		}
		if minLength, ok := config["min_length"].(float64); ok {
			if minLength < 0 {
				log.Error("Compression min_length cannot be negative")
				return fmt.Errorf("compression min_length cannot be negative")
			}
		}
	}

	return nil
}

// validateTLSConfig validates TLS configuration
func validateTLSConfig(config *TLSConfig, log *zap.Logger) error {
	if !config.Enabled {
		log.Info("TLS is disabled")
		return nil
	}

	if config.AutoCert.Enabled {
		if config.AutoCert.Email == "" {
			log.Error("Let's Encrypt email cannot be empty")
			return fmt.Errorf("Let's Encrypt email cannot be empty")
		}

		if len(config.AutoCert.Hosts) == 0 {
			log.Error("At least one host must be specified for Let's Encrypt")
			return fmt.Errorf("at least one host must be specified for Let's Encrypt")
		}

		for _, host := range config.AutoCert.Hosts {
			if host == "" {
				log.Error("Let's Encrypt host cannot be empty")
				return fmt.Errorf("Let's Encrypt host cannot be empty")
			}
		}

		if config.AutoCert.CacheDir == "" {
			log.Error("Let's Encrypt cache directory cannot be empty")
			return fmt.Errorf("Let's Encrypt cache directory cannot be empty")
		}
	}

	for i, cert := range config.Certificates {
		if len(cert.Hosts) == 0 {
			log.Error("Certificate must have at least one host", zap.Int("certificate", i))
			return fmt.Errorf("certificate %d must have at least one host", i)
		}

		if cert.CertFile == "" {
			log.Error("Certificate cert file cannot be empty", zap.Int("certificate", i))
			return fmt.Errorf("certificate %d cert file cannot be empty", i)
		}

		if cert.KeyFile == "" {
			log.Error("Certificate key file cannot be empty", zap.Int("certificate", i))
			return fmt.Errorf("certificate %d key file cannot be empty", i)
		}
	}

	return nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
