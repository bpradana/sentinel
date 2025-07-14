package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"go.uber.org/zap"
)

// CertificateGenerator handles automatic certificate generation
type CertificateGenerator struct {
	logger *zap.Logger
}

// NewCertificateGenerator creates a new certificate generator
func NewCertificateGenerator(logger *zap.Logger) *CertificateGenerator {
	return &CertificateGenerator{
		logger: logger,
	}
}

// SelfSignedCertConfig holds configuration for self-signed certificate generation
type SelfSignedCertConfig struct {
	Hosts      []string      // DNS names and IP addresses
	ValidFor   time.Duration // Certificate validity period
	RSABits    int           // RSA key size
	CertFile   string        // Output certificate file path
	KeyFile    string        // Output private key file path
	CommonName string        // Certificate common name
	Country    []string      // Country
	Province   []string      // Province/State
	City       []string      // City/Locality
	Org        []string      // Organization
	OrgUnit    []string      // Organizational Unit
}

// GenerateSelfSignedCertificate generates a self-signed certificate and private key
func (g *CertificateGenerator) GenerateSelfSignedCertificate(config *SelfSignedCertConfig) error {
	// Set defaults
	if config.ValidFor == 0 {
		config.ValidFor = 365 * 24 * time.Hour // 1 year
	}
	if config.RSABits == 0 {
		config.RSABits = 2048
	}
	if config.CommonName == "" && len(config.Hosts) > 0 {
		config.CommonName = config.Hosts[0]
	}

	g.logger.Info("Generating self-signed certificate",
		zap.Strings("hosts", config.Hosts),
		zap.Duration("valid_for", config.ValidFor),
		zap.Int("rsa_bits", config.RSABits))

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, config.RSABits)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         config.CommonName,
			Country:            config.Country,
			Province:           config.Province,
			Locality:           config.City,
			Organization:       config.Org,
			OrganizationalUnit: config.OrgUnit,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(config.ValidFor),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add DNS names and IP addresses
	for _, host := range config.Hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate file
	if err := g.writeCertificateFile(config.CertFile, certDER); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	// Write private key file
	if err := g.writePrivateKeyFile(config.KeyFile, privateKey); err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}

	g.logger.Info("Self-signed certificate generated successfully",
		zap.String("cert_file", config.CertFile),
		zap.String("key_file", config.KeyFile))

	return nil
}

// writeCertificateFile writes the certificate to a PEM file
func (g *CertificateGenerator) writeCertificateFile(filename string, certDER []byte) error {
	certFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer certFile.Close()

	// Set restrictive permissions
	if err := certFile.Chmod(0644); err != nil {
		return err
	}

	return pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// writePrivateKeyFile writes the private key to a PEM file
func (g *CertificateGenerator) writePrivateKeyFile(filename string, privateKey *rsa.PrivateKey) error {
	keyFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer keyFile.Close()

	// Set restrictive permissions for private key
	if err := keyFile.Chmod(0600); err != nil {
		return err
	}

	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	return pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER})
}

// GenerateCSR generates a Certificate Signing Request
func (g *CertificateGenerator) GenerateCSR(config *SelfSignedCertConfig) ([]byte, *rsa.PrivateKey, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, config.RSABits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create CSR template
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         config.CommonName,
			Country:            config.Country,
			Province:           config.Province,
			Locality:           config.City,
			Organization:       config.Org,
			OrganizationalUnit: config.OrgUnit,
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	// Add DNS names and IP addresses
	for _, host := range config.Hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	// Create CSR
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	return csrDER, privateKey, nil
}

// CheckCertificateExists checks if certificate files exist and are valid
func (g *CertificateGenerator) CheckCertificateExists(certFile, keyFile string) bool {
	// Check if files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return false
	}

	// Try to load and validate the certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		g.logger.Warn("Certificate files exist but are invalid", zap.Error(err))
		return false
	}

	// Parse certificate to check expiration
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		g.logger.Warn("Failed to parse certificate", zap.Error(err))
		return false
	}

	// Check if certificate is expired or expires soon (within 30 days)
	if time.Now().After(x509Cert.NotAfter) || time.Now().Add(30*24*time.Hour).After(x509Cert.NotAfter) {
		g.logger.Info("Certificate expired or expires soon", zap.Time("expires", x509Cert.NotAfter))
		return false
	}

	return true
}

// AutoGenerateCertificate automatically generates a certificate if it doesn't exist or is invalid
func (g *CertificateGenerator) AutoGenerateCertificate(config *SelfSignedCertConfig) error {
	// Check if certificate already exists and is valid
	if g.CheckCertificateExists(config.CertFile, config.KeyFile) {
		g.logger.Info("Valid certificate already exists",
			zap.String("cert_file", config.CertFile),
			zap.String("key_file", config.KeyFile))
		return nil
	}

	// Generate new certificate
	g.logger.Info("Generating new certificate")
	return g.GenerateSelfSignedCertificate(config)
}
