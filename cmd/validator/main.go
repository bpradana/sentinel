package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bpradana/sentinel/internal/config"
	"github.com/bpradana/sentinel/pkg/logger"
)

func main() {
	var configDir = flag.String("config", "./config", "Configuration directory")
	var logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	var verbose = flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Initialize logger
	log, err := logger.NewLogger(*logLevel)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	fmt.Println("🔍 Sentinel Configuration Validator")
	fmt.Println("====================================")

	// Check if config directory exists
	if _, err := os.Stat(*configDir); os.IsNotExist(err) {
		fmt.Printf("❌ Configuration directory does not exist: %s\n", *configDir)
		os.Exit(1)
	}

	fmt.Printf("📁 Validating configuration in: %s\n\n", *configDir)

	// Load configuration
	cfg, err := config.LoadConfig(*configDir)
	if err != nil {
		fmt.Printf("❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Configuration files loaded successfully")

	// Validate configuration
	if err := config.ValidateConfig(cfg, log); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Configuration validation passed")

	// Print configuration summary if verbose
	if *verbose {
		printConfigurationSummary(cfg)
	}

	fmt.Println("\n🎉 All validations passed! Your configuration is ready to use.")
}

func printConfigurationSummary(cfg *config.Config) {
	fmt.Println("\n📊 Configuration Summary:")
	fmt.Println("------------------------")

	// Global settings
	fmt.Printf("🌐 Global Settings:\n")
	fmt.Printf("  HTTP Port: %d\n", cfg.Global.Server.HTTPPort)
	fmt.Printf("  HTTPS Port: %d\n", cfg.Global.Server.HTTPSPort)
	fmt.Printf("  Read Timeout: %v\n", cfg.Global.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %v\n", cfg.Global.Server.WriteTimeout)
	fmt.Printf("  Idle Timeout: %v\n", cfg.Global.Server.IdleTimeout)
	fmt.Printf("  HTTP/2 Enabled: %t\n", cfg.Global.Server.HTTP2Enabled)
	fmt.Printf("  Log Level: %s\n", cfg.Global.Log.Level)
	fmt.Printf("  Log Format: %s\n", cfg.Global.Log.Format)

	// Upstreams
	fmt.Printf("\n🔄 Upstream Services (%d):\n", len(cfg.Upstreams.Services))
	for name, service := range cfg.Upstreams.Services {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Load Balancer: %s\n", service.LoadBalancer)
		fmt.Printf("    Targets: %d\n", len(service.Targets))
		fmt.Printf("    Health Check: %t\n", service.HealthCheck.Enabled)
	}

	// Routes
	fmt.Printf("\n🛣️  Routes (%d):\n", len(cfg.Routes.Rules))
	for i, rule := range cfg.Routes.Rules {
		fmt.Printf("  %d. %s%s -> %s\n", i+1, rule.Host, rule.Path, rule.Upstream)
	}

	// Middleware
	fmt.Printf("\n🔧 Middleware Chains (%d):\n", len(cfg.Middleware.Chain))
	for _, chain := range cfg.Middleware.Chain {
		if chain.Enabled {
			fmt.Printf("  %s (%s) - Order: %d\n", chain.Name, chain.Type, chain.Order)
		}
	}

	// TLS
	fmt.Printf("\n🔒 TLS Configuration:\n")
	fmt.Printf("  Enabled: %t\n", cfg.TLS.Enabled)
	if cfg.TLS.Enabled {
		fmt.Printf("  Auto-cert: %t\n", cfg.TLS.AutoCert.Enabled)
		fmt.Printf("  Manual Certificates: %d\n", len(cfg.TLS.Certificates))
	}

	// Health
	fmt.Printf("\n💚 Health Check:\n")
	fmt.Printf("  Enabled: %t\n", cfg.Health.Enabled)
	if cfg.Health.Enabled {
		fmt.Printf("  Port: %d\n", cfg.Health.Port)
		fmt.Printf("  Interval: %v\n", cfg.Health.Interval)
		fmt.Printf("  Timeout: %v\n", cfg.Health.Timeout)
	}

	// Metrics
	fmt.Printf("\n📈 Metrics:\n")
	fmt.Printf("  Enabled: %t\n", cfg.Metrics.Enabled)
	if cfg.Metrics.Enabled {
		fmt.Printf("  Port: %d\n", cfg.Metrics.Port)
		fmt.Printf("  Path: %s\n", cfg.Metrics.Path)
	}
}
