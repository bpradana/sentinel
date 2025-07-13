package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var (
		hosts      = flag.String("hosts", "localhost,127.0.0.1", "Comma-separated list of hosts")
		outputDir  = flag.String("output", "./certs", "Output directory for certificates")
		days       = flag.Int("days", 365, "Certificate validity in days")
		keySize    = flag.Int("key-size", 2048, "RSA key size in bits")
		commonName = flag.String("cn", "Sentinel Development Certificate", "Common name for the certificate")
		org        = flag.String("org", "Sentinel Development", "Organization name")
		country    = flag.String("country", "US", "Country code")
		state      = flag.String("state", "Development", "State or province")
		city       = flag.String("city", "Development", "City")
	)
	flag.Parse()

	fmt.Println("🔐 Sentinel Self-Signed Certificate Generator")
	fmt.Println("=============================================")

	// Parse hosts
	hostList := strings.Split(*hosts, ",")
	for i, host := range hostList {
		hostList[i] = strings.TrimSpace(host)
	}

	fmt.Printf("📋 Generating certificate for hosts: %s\n", strings.Join(hostList, ", "))
	fmt.Printf("📁 Output directory: %s\n", *outputDir)
	fmt.Printf("⏰ Validity: %d days\n", *days)

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate private key
	fmt.Println("\n🔑 Generating RSA private key...")
	privateKey, err := rsa.GenerateKey(rand.Reader, *keySize)
	if err != nil {
		fmt.Printf("❌ Failed to generate private key: %v\n", err)
		os.Exit(1)
	}

	// Create certificate template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		fmt.Printf("❌ Failed to generate serial number: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{*country},
			Organization:       []string{*org},
			OrganizationalUnit: []string{"Development"},
			Locality:           []string{*city},
			Province:           []string{*state},
			CommonName:         *commonName,
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, *days),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{},
		IPAddresses:           []net.IP{},
	}

	// Add hosts to certificate
	for _, host := range hostList {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	// Create certificate
	fmt.Println("📜 Creating certificate...")
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		fmt.Printf("❌ Failed to create certificate: %v\n", err)
		os.Exit(1)
	}

	// Write certificate file
	certFile := filepath.Join(*outputDir, "cert.pem")
	certOut, err := os.Create(certFile)
	if err != nil {
		fmt.Printf("❌ Failed to create certificate file: %v\n", err)
		os.Exit(1)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		fmt.Printf("❌ Failed to write certificate: %v\n", err)
		os.Exit(1)
	}

	// Write private key file
	keyFile := filepath.Join(*outputDir, "key.pem")
	keyOut, err := os.Create(keyFile)
	if err != nil {
		fmt.Printf("❌ Failed to create key file: %v\n", err)
		os.Exit(1)
	}
	defer keyOut.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		fmt.Printf("❌ Failed to write private key: %v\n", err)
		os.Exit(1)
	}

	// Validate the certificate
	fmt.Println("🔍 Validating generated certificate...")
	if err := validateCertificate(certFile, keyFile, hostList); err != nil {
		fmt.Printf("❌ Certificate validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Certificate generated successfully!")
	fmt.Printf("📄 Certificate: %s\n", certFile)
	fmt.Printf("🔑 Private Key: %s\n", keyFile)
	fmt.Printf("⏰ Valid until: %s\n", template.NotAfter.Format("2006-01-02 15:04:05"))

	fmt.Println("\n📝 Next steps:")
	fmt.Println("1. Update your TLS configuration to use these certificates")
	fmt.Println("2. Add the certificate files to your .gitignore")
	fmt.Println("3. For production, use proper CA-signed certificates")

	// Generate example TLS config
	generateExampleConfig(*outputDir, hostList)
}

func validateCertificate(certFile, keyFile string, hosts []string) error {
	// Load certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	// Parse certificate
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check expiration
	if time.Now().After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate is expired")
	}

	if time.Now().Before(x509Cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid")
	}

	// Check hosts
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			found := false
			for _, certIP := range x509Cert.IPAddresses {
				if certIP.Equal(ip) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("IP address %s not found in certificate", host)
			}
		} else {
			found := false
			for _, dnsName := range x509Cert.DNSNames {
				if dnsName == host {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("DNS name %s not found in certificate", host)
			}
		}
	}

	return nil
}

func generateExampleConfig(outputDir string, hosts []string) {
	exampleFile := filepath.Join(outputDir, "tls-example.yaml")
	content := fmt.Sprintf(`# Example TLS configuration for Sentinel
enabled: true

autocert:
  enabled: false
  email: "admin@example.com"
  hosts:
    - "%s"
  cache_dir: "./certs"
  staging: true

certificates:
  - hosts:
%s
    cert_file: "%s/cert.pem"
    key_file: "%s/key.pem"
`, strings.Join(hosts, `"`+"\n    - "+`"`),
		func() string {
			var result []string
			for _, host := range hosts {
				result = append(result, fmt.Sprintf("      - \"%s\"", host))
			}
			return strings.Join(result, "\n")
		}(),
		outputDir, outputDir)

	if err := os.WriteFile(exampleFile, []byte(content), 0644); err != nil {
		fmt.Printf("⚠️  Failed to create example config: %v\n", err)
		return
	}

	fmt.Printf("📄 Example TLS config: %s\n", exampleFile)
}
