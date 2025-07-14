package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bpradana/sentinel/internal/config"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Manager handles TLS certificate management
type Manager struct {
	cfg          *config.TLSConfig
	logger       *zap.Logger
	autocertMgr  *autocert.Manager
	certificates map[string]*tls.Certificate
	mu           sync.RWMutex
	generator    *CertificateGenerator
}

// NewManager creates a new TLS manager
func NewManager(cfg *config.TLSConfig, logger *zap.Logger) (*Manager, error) {
	if !cfg.Enabled {
		logger.Info("TLS is disabled")
		return &Manager{
			cfg:          cfg,
			logger:       logger,
			certificates: make(map[string]*tls.Certificate),
		}, nil
	}

	manager := &Manager{
		cfg:          cfg,
		logger:       logger,
		certificates: make(map[string]*tls.Certificate),
		generator:    NewCertificateGenerator(logger),
	}

	// Initialize auto-cert manager if enabled
	if cfg.AutoCert.Enabled {
		if err := manager.initAutoCert(); err != nil {
			return nil, fmt.Errorf("failed to initialize auto-cert: %w", err)
		}
	}

	// Load manual certificates
	if err := manager.loadManualCertificates(); err != nil {
		return nil, fmt.Errorf("failed to load manual certificates: %w", err)
	}

	return manager, nil
}

// initAutoCert initializes the Let's Encrypt auto-cert manager
func (m *Manager) initAutoCert() error {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(m.cfg.AutoCert.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Configure auto-cert manager
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(m.cfg.AutoCert.CacheDir),
		HostPolicy: autocert.HostWhitelist(m.cfg.AutoCert.Hosts...),
	}

	// Set email for Let's Encrypt account
	if m.cfg.AutoCert.Email != "" {
		m.autocertMgr.Email = m.cfg.AutoCert.Email
	}

	// Configure staging environment if enabled
	if m.cfg.AutoCert.Staging {
		// Create ACME client with staging directory
		m.autocertMgr.Client = &acme.Client{
			DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
		}
		m.logger.Info("Using Let's Encrypt staging environment")
	}

	m.logger.Info("Auto-cert manager initialized",
		zap.Strings("hosts", m.cfg.AutoCert.Hosts),
		zap.String("cache_dir", m.cfg.AutoCert.CacheDir),
		zap.Bool("staging", m.cfg.AutoCert.Staging))

	return nil
}

// loadManualCertificates loads manually configured certificates
func (m *Manager) loadManualCertificates() error {
	for i, certConfig := range m.cfg.Certificates {
		if err := m.loadCertificate(&certConfig); err != nil {
			return fmt.Errorf("failed to load certificate %d: %w", i, err)
		}
	}
	return nil
}

// loadCertificate loads a single certificate
func (m *Manager) loadCertificate(certConfig *config.CertificateConfig) error {
	// If auto-generate is enabled, check if we need to generate certificates
	if certConfig.AutoGenerate {
		if err := m.ensureCertificateExists(certConfig); err != nil {
			return fmt.Errorf("failed to ensure certificate exists: %w", err)
		}
	}

	// Read certificate file
	certPEM, err := os.ReadFile(certConfig.CertFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Read private key file
	keyPEM, err := os.ReadFile(certConfig.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Parse certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Validate certificate
	if err := m.validateCertificate(&cert); err != nil {
		return fmt.Errorf("certificate validation failed: %w", err)
	}

	// Store certificate for each host
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, host := range certConfig.Hosts {
		m.certificates[host] = &cert
		m.logger.Info("Loaded certificate",
			zap.String("host", host),
			zap.String("cert_file", certConfig.CertFile),
			zap.String("key_file", certConfig.KeyFile),
			zap.Bool("auto_generated", certConfig.AutoGenerate))
	}

	return nil
}

// ensureCertificateExists generates certificates if they don't exist or are invalid
func (m *Manager) ensureCertificateExists(certConfig *config.CertificateConfig) error {
	// Check if certificate already exists and is valid
	if m.generator.CheckCertificateExists(certConfig.CertFile, certConfig.KeyFile) {
		return nil
	}

	// Only generate self-signed certificates for now
	if !certConfig.SelfSigned {
		return fmt.Errorf("auto-generate is enabled but self_signed is false - only self-signed certificates can be auto-generated")
	}

	// Parse validity duration
	validFor := 365 * 24 * time.Hour // default 1 year
	if certConfig.ValidFor != "" {
		duration, err := time.ParseDuration(certConfig.ValidFor)
		if err != nil {
			return fmt.Errorf("invalid valid_for duration: %w", err)
		}
		validFor = duration
	}

	// Set default RSA bits
	rsaBits := certConfig.RSABits
	if rsaBits == 0 {
		rsaBits = 2048
	}

	// Create certificate generation config
	genConfig := &SelfSignedCertConfig{
		Hosts:      certConfig.Hosts,
		ValidFor:   validFor,
		RSABits:    rsaBits,
		CertFile:   certConfig.CertFile,
		KeyFile:    certConfig.KeyFile,
		CommonName: certConfig.CommonName,
		Org:        []string{certConfig.Organization},
	}

	// Generate certificate
	return m.generator.GenerateSelfSignedCertificate(genConfig)
}

// validateCertificate validates a certificate
func (m *Manager) validateCertificate(cert *tls.Certificate) error {
	// Parse the certificate to check expiration
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if certificate is expired
	if time.Now().After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate expired at %v", x509Cert.NotAfter)
	}

	// Check if certificate is not yet valid
	if time.Now().Before(x509Cert.NotBefore) {
		return fmt.Errorf("certificate not valid until %v", x509Cert.NotBefore)
	}

	// Log certificate details
	m.logger.Debug("Certificate validated",
		zap.String("subject", x509Cert.Subject.CommonName),
		zap.Strings("dns_names", x509Cert.DNSNames),
		zap.Time("not_before", x509Cert.NotBefore),
		zap.Time("not_after", x509Cert.NotAfter))

	return nil
}

// GetTLSConfig returns a TLS configuration for the given host
func (m *Manager) GetTLSConfig(host string) (*tls.Config, error) {
	if !m.cfg.Enabled {
		return nil, fmt.Errorf("TLS is disabled")
	}

	// Create a TLS config with a certificate callback
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Strip port from hostname if present
			requestedHost := clientHello.ServerName
			if colonIndex := strings.Index(requestedHost, ":"); colonIndex != -1 {
				requestedHost = requestedHost[:colonIndex]
			}

			// If no hostname provided, use the first available certificate
			if requestedHost == "" {
				m.mu.RLock()
				defer m.mu.RUnlock()
				for _, cert := range m.certificates {
					return cert, nil
				}
				return nil, fmt.Errorf("no default certificate available")
			}

			// Check if we have a manual certificate for this host
			m.mu.RLock()
			defer m.mu.RUnlock()
			if cert, exists := m.certificates[requestedHost]; exists {
				return cert, nil
			}

			// If auto-cert is enabled, use it
			if m.cfg.AutoCert.Enabled && m.autocertMgr != nil {
				return m.autocertMgr.GetCertificate(clientHello)
			}

			return nil, fmt.Errorf("no certificate found for host: %s", requestedHost)
		},
	}

	return tlsConfig, nil
}

// GetAutoCertManager returns the auto-cert manager if available
func (m *Manager) GetAutoCertManager() *autocert.Manager {
	return m.autocertMgr
}

// ReloadCertificates reloads all manual certificates
func (m *Manager) ReloadCertificates() error {
	m.logger.Info("Reloading manual certificates")

	// Clear existing certificates
	m.mu.Lock()
	m.certificates = make(map[string]*tls.Certificate)
	m.mu.Unlock()

	// Reload certificates
	return m.loadManualCertificates()
}

// GetCertificateInfo returns information about certificates
func (m *Manager) GetCertificateInfo() map[string]any {
	info := map[string]any{
		"enabled": m.cfg.Enabled,
		"autocert": map[string]any{
			"enabled":   m.cfg.AutoCert.Enabled,
			"hosts":     m.cfg.AutoCert.Hosts,
			"cache_dir": m.cfg.AutoCert.CacheDir,
			"staging":   m.cfg.AutoCert.Staging,
		},
		"manual_certificates": len(m.cfg.Certificates),
	}

	m.mu.RLock()
	hosts := make([]string, 0, len(m.certificates))
	for host := range m.certificates {
		hosts = append(hosts, host)
	}
	m.mu.RUnlock()

	info["certified_hosts"] = hosts

	return info
}

// ValidateHost checks if a host is supported by TLS
func (m *Manager) ValidateHost(host string) bool {
	if !m.cfg.Enabled {
		return false
	}

	// Check manual certificates
	m.mu.RLock()
	if _, exists := m.certificates[host]; exists {
		m.mu.RUnlock()
		return true
	}
	m.mu.RUnlock()

	// Check auto-cert hosts
	if m.cfg.AutoCert.Enabled {
		for _, allowedHost := range m.cfg.AutoCert.Hosts {
			if host == allowedHost {
				return true
			}
		}
	}

	return false
}

// RegenerateCertificates regenerates certificates
func (m *Manager) RegenerateCertificates() error {
	m.logger.Info("Regenerating certificates")

	for i, certConfig := range m.cfg.Certificates {
		if !certConfig.AutoGenerate || !certConfig.SelfSigned {
			continue
		}

		// Force regeneration by removing existing files
		os.Remove(certConfig.CertFile)
		os.Remove(certConfig.KeyFile)

		if err := m.ensureCertificateExists(&certConfig); err != nil {
			return fmt.Errorf("failed to regenerate certificate %d: %w", i, err)
		}
	}

	// Reload certificates
	return m.ReloadCertificates()
}

// Shutdown performs cleanup operations
func (m *Manager) Shutdown() error {
	m.logger.Info("Shutting down TLS manager")
	// No specific cleanup needed for TLS manager
	return nil
}
