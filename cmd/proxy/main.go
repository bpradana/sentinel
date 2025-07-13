package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bpradana/sentinel/internal/config"
	"github.com/bpradana/sentinel/internal/health"
	"github.com/bpradana/sentinel/internal/metrics"
	"github.com/bpradana/sentinel/internal/proxy"
	"github.com/bpradana/sentinel/internal/tls"
	"github.com/bpradana/sentinel/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

func main() {
	var configDir = flag.String("config", "./configs/default", "Configuration directory")
	var logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logger
	log, err := logger.NewLogger(*logLevel)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Load configuration
	cfg, err := config.LoadConfig(*configDir)
	if err != nil {
		log.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg, log); err != nil {
		log.Fatal("Configuration validation failed", zap.Error(err))
	}

	log.Info("Configuration loaded successfully", zap.String("config_dir", *configDir))

	// Initialize TLS manager
	tlsManager, err := tls.NewManager(&cfg.TLS, log)
	if err != nil {
		log.Fatal("Failed to initialize TLS manager", zap.Error(err))
	}

	// Initialize health checker
	healthChecker := health.NewChecker(cfg.Health, log)

	// Initialize metrics
	metricsServer := metrics.NewServer(&cfg.Metrics, log)
	go func() {
		if err := metricsServer.Start(); err != nil {
			log.Error("Failed to start metrics server", zap.Error(err))
		}
	}()

	// Initialize proxy server
	proxyServer := proxy.NewServer(cfg, tlsManager, healthChecker, log)

	// Start health monitoring
	healthChecker.Start()

	// Start proxy server
	go func() {
		if err := proxyServer.Start(); err != nil {
			log.Error("Failed to start proxy server", zap.Error(err))
		}
	}()

	// Setup configuration hot-reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to create file watcher", zap.Error(err))
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Info("Configuration file changed, reloading...", zap.String("file", event.Name))
					if newCfg, err := config.LoadConfig(*configDir); err == nil {
						if err := config.ValidateConfig(newCfg, log); err == nil {
							proxyServer.UpdateConfig(newCfg)
							log.Info("Configuration reloaded successfully")
						} else {
							log.Error("Configuration validation failed during reload", zap.Error(err))
						}
					} else {
						log.Error("Failed to reload configuration", zap.Error(err))
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("File watcher error", zap.Error(err))
			}
		}
	}()

	if err := watcher.Add(*configDir); err != nil {
		log.Error("Failed to add config directory to watcher", zap.Error(err))
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown components
	healthChecker.Stop()
	metricsServer.Stop()

	if err := proxyServer.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server shutdown complete")
}
