package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CertDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sushiro-proxy")
}

func LoadOrGenerateCA() (tls.Certificate, *rsa.PrivateKey, error) {
	dir := CertDirPath()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	// Try to load existing
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if certErr == nil && keyErr == nil {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err == nil {
			block, _ := pem.Decode(certPEM)
			if block != nil {
				parsed, err := x509.ParseCertificate(block.Bytes)
				if err == nil && time.Now().Before(parsed.NotAfter) {
					keyBlock, _ := pem.Decode(keyPEM)
					if keyBlock != nil {
						key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
						if err == nil {
							return cert, key, nil
						}
					}
				}
			}
		}
	}

	// Generate new CA
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("create cert dir: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("generate RSA key: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "Sushiro Proxy CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("create CA cert: %w", err)
	}

	certPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	if err := os.WriteFile(certPath, certPEMBytes, 0o644); err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("write CA cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEMBytes, 0o600); err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("write CA key: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEMBytes, keyPEMBytes)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load generated cert: %w", err)
	}

	fmt.Println("CA证书已生成:", certPath)
	return cert, key, nil
}

func loadLocalCACertificate() (*x509.Certificate, error) {
	certPath := filepath.Join(CertDirPath(), "ca.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("decode CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func LocalCACertSHA1Thumbprint() (string, error) {
	cert, err := loadLocalCACertificate()
	if err != nil {
		return "", err
	}
	return certSHA1Thumbprint(cert), nil
}

func certSHA1Thumbprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func generateHostCert(caTLSCert tls.Certificate, caKey *rsa.PrivateKey, host string) (tls.Certificate, error) {
	caCert, err := x509.ParseCertificate(caTLSCert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse CA cert: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	h := host
	if ip, _, err := net.SplitHostPort(host); err == nil {
		h = ip
	}
	if ip := net.ParseIP(h); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{h}
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate leaf key: %w", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create leaf cert: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER, caCert.Raw},
		PrivateKey:  leafKey,
	}, nil
}
