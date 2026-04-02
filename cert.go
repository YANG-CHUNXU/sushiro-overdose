package main

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
	"os/exec"
	"path/filepath"
	"time"
)

func certDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sushiro-proxy")
}

func loadOrGenerateCA() (tls.Certificate, *rsa.PrivateKey, error) {
	dir := certDirPath()
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

func isCertTrusted() (bool, error) {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	if _, err := os.Stat(certPath); err != nil {
		return false, nil
	}

	cmd := exec.Command("security", "verify-cert", "-c", certPath, "-p", "basic")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func installCert() error {
	dir := certDirPath()
	certPath := filepath.Join(dir, "ca.crt")

	// Add cert to user login keychain
	cmd := exec.Command("security", "add-certificates", "-k",
		filepath.Join(os.Getenv("HOME"), "Library/Keychains/login.keychain-db"),
		certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add-certificates: %w", err)
	}

	// Set trust at user level (no sudo needed on macOS 26+)
	cmd = exec.Command("security", "add-trusted-cert", "-r", "trustRoot", certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		Certificate: [][]byte{certDER},
		PrivateKey:  leafKey,
	}, nil
}
