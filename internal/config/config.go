package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Global     GlobalConfig     `yaml:"global"`
	Upstreams  UpstreamsConfig  `yaml:"upstreams"`
	Routes     RoutesConfig     `yaml:"routes"`
	Middleware MiddlewareConfig `yaml:"middleware"`
	TLS        TLSConfig        `yaml:"tls"`
	Health     HealthConfig     `yaml:"health"`
	Metrics    MetricsConfig    `yaml:"metrics"`
}

// GlobalConfig holds global server settings
type GlobalConfig struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
}

// ServerConfig defines server-specific settings
type ServerConfig struct {
	HTTPPort      int           `yaml:"http_port"`
	HTTPSPort     int           `yaml:"https_port"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	IdleTimeout   time.Duration `yaml:"idle_timeout"`
	MaxHeaderSize int           `yaml:"max_header_size"`
	HTTP2Enabled  bool          `yaml:"http2_enabled"`
}

// LogConfig defines logging settings
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// UpstreamsConfig defines upstream service configurations
type UpstreamsConfig struct {
	Services map[string]UpstreamService `yaml:"services"`
}

// UpstreamService defines a single upstream service
type UpstreamService struct {
	LoadBalancer string            `yaml:"load_balancer"`
	HealthCheck  HealthCheckConfig `yaml:"health_check"`
	Targets      []Target          `yaml:"targets"`
}

// Target defines an upstream target
type Target struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight,omitempty"`
}

// HealthCheckConfig defines health check settings
type HealthCheckConfig struct {
	Enabled          bool          `yaml:"enabled"`
	Path             string        `yaml:"path"`
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
}

// RoutesConfig defines routing rules
type RoutesConfig struct {
	Rules []RouteRule `yaml:"rules"`
}

// RouteRule defines a single routing rule
type RouteRule struct {
	Host        string            `yaml:"host"`
	Path        string            `yaml:"path"`
	Methods     []string          `yaml:"methods,omitempty"`
	Upstream    string            `yaml:"upstream"`
	Rewrite     RewriteConfig     `yaml:"rewrite,omitempty"`
	Middleware  []string          `yaml:"middleware,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Timeout     time.Duration     `yaml:"timeout,omitempty"`
	RetryPolicy RetryPolicy       `yaml:"retry_policy,omitempty"`
}

// RewriteConfig defines URL rewriting rules
type RewriteConfig struct {
	StripPrefix string `yaml:"strip_prefix,omitempty"`
	AddPrefix   string `yaml:"add_prefix,omitempty"`
	Regex       string `yaml:"regex,omitempty"`
	Replacement string `yaml:"replacement,omitempty"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	Attempts int           `yaml:"attempts"`
	Backoff  time.Duration `yaml:"backoff"`
}

// MiddlewareConfig defines middleware configurations
type MiddlewareConfig struct {
	Chain []MiddlewareChain `yaml:"chain"`
}

// MiddlewareChain defines a middleware chain
type MiddlewareChain struct {
	Name    string         `yaml:"name"`
	Type    string         `yaml:"type"`
	Config  map[string]any `yaml:"config,omitempty"`
	Enabled bool           `yaml:"enabled"`
	Order   int            `yaml:"order"`
}

// TLSConfig defines TLS settings
type TLSConfig struct {
	Enabled      bool                `yaml:"enabled"`
	AutoCert     AutoCertConfig      `yaml:"autocert"`
	Certificates []CertificateConfig `yaml:"certificates,omitempty"`
}

// AutoCertConfig defines Let's Encrypt configuration
type AutoCertConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Email    string   `yaml:"email"`
	Hosts    []string `yaml:"hosts"`
	CacheDir string   `yaml:"cache_dir"`
	Staging  bool     `yaml:"staging"`
}

// CertificateConfig defines manual certificate configuration
type CertificateConfig struct {
	Hosts    []string `yaml:"hosts"`
	CertFile string   `yaml:"cert_file"`
	KeyFile  string   `yaml:"key_file"`
}

// HealthConfig defines health check settings
type HealthConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Port     int           `yaml:"port"`
}

// MetricsConfig defines metrics settings
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// LoadConfig loads configuration from the specified directory
func LoadConfig(configDir string) (*Config, error) {
	config := &Config{}

	// Load global configuration
	if err := loadYAMLFile(filepath.Join(configDir, "global.yaml"), &config.Global); err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	// Load upstreams configuration
	if err := loadYAMLFile(filepath.Join(configDir, "upstreams.yaml"), &config.Upstreams); err != nil {
		return nil, fmt.Errorf("failed to load upstreams config: %w", err)
	}

	// Load routes configuration
	if err := loadYAMLFile(filepath.Join(configDir, "routes.yaml"), &config.Routes); err != nil {
		return nil, fmt.Errorf("failed to load routes config: %w", err)
	}

	// Load middleware configuration
	if err := loadYAMLFile(filepath.Join(configDir, "middleware.yaml"), &config.Middleware); err != nil {
		return nil, fmt.Errorf("failed to load middleware config: %w", err)
	}

	// Load TLS configuration
	if err := loadYAMLFile(filepath.Join(configDir, "tls.yaml"), &config.TLS); err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}

	// Load health configuration
	if err := loadYAMLFile(filepath.Join(configDir, "health.yaml"), &config.Health); err != nil {
		return nil, fmt.Errorf("failed to load health config: %w", err)
	}

	// Load metrics configuration
	if err := loadYAMLFile(filepath.Join(configDir, "metrics.yaml"), &config.Metrics); err != nil {
		return nil, fmt.Errorf("failed to load metrics config: %w", err)
	}

	// Set defaults
	setDefaults(config)

	return config, nil
}

// loadYAMLFile loads a YAML file into the provided structure
func loadYAMLFile(filename string, v any) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, v)
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Global.Server.HTTPPort == 0 {
		config.Global.Server.HTTPPort = 8080
	}
	if config.Global.Server.HTTPSPort == 0 {
		config.Global.Server.HTTPSPort = 8443
	}
	if config.Global.Server.ReadTimeout == 0 {
		config.Global.Server.ReadTimeout = 30 * time.Second
	}
	if config.Global.Server.WriteTimeout == 0 {
		config.Global.Server.WriteTimeout = 30 * time.Second
	}
	if config.Global.Server.IdleTimeout == 0 {
		config.Global.Server.IdleTimeout = 60 * time.Second
	}
	if config.Global.Server.MaxHeaderSize == 0 {
		config.Global.Server.MaxHeaderSize = 1024 * 1024 // 1MB
	}
	if config.Global.Log.Level == "" {
		config.Global.Log.Level = "info"
	}
	if config.Global.Log.Format == "" {
		config.Global.Log.Format = "json"
	}
	if config.Health.Interval == 0 {
		config.Health.Interval = 30 * time.Second
	}
	if config.Health.Timeout == 0 {
		config.Health.Timeout = 5 * time.Second
	}
	if config.Health.Port == 0 {
		config.Health.Port = 8081
	}
	if config.Metrics.Port == 0 {
		config.Metrics.Port = 8082
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}
	if config.TLS.AutoCert.CacheDir == "" {
		config.TLS.AutoCert.CacheDir = "./certs"
	}
}
