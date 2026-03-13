package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Manage mTLS certificates",
}

var certInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create CA and client certificates in ~/.kaptan/certs/",
	RunE:  runCertInit,
}

var certRotateCmd = &cobra.Command{
	Use:   "rotate --server=<name>",
	Short: "Rotate client certificate for a server",
	RunE:  runCertRotate,
}

var certRotateServer string

func init() {
	certCmd.AddCommand(certInitCmd)
	certCmd.AddCommand(certRotateCmd)
	certRotateCmd.Flags().StringVar(&certRotateServer, "server", "", "server to rotate cert for")
}

func runCertInit(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	certsDir := filepath.Join(home, ".kaptan", "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return err
	}

	// Generate CA
	caKey, caCert, caCertDER, err := generateCA()
	if err != nil {
		return fmt.Errorf("generate CA: %w", err)
	}

	if err := writePEM(filepath.Join(certsDir, "ca.key"), "EC PRIVATE KEY", ecKeyBytes(caKey)); err != nil {
		return err
	}
	if err := writePEM(filepath.Join(certsDir, "ca.crt"), "CERTIFICATE", caCertDER); err != nil {
		return err
	}
	fmt.Println("Created CA: ~/.kaptan/certs/ca.{crt,key}")

	// Generate client cert signed by CA
	if err := generateClientCert(certsDir, caKey, caCert); err != nil {
		return fmt.Errorf("generate client cert: %w", err)
	}
	fmt.Println("Created client cert: ~/.kaptan/certs/client.{crt,key}")

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run: m server bootstrap <name> <ssh-user@host>")
	fmt.Println("  2. The bootstrap will install the agent and copy ~/.kaptan/certs/ca.crt")

	return nil
}

func runCertRotate(cmd *cobra.Command, args []string) error {
	if certRotateServer == "" {
		return fmt.Errorf("--server is required")
	}
	home, _ := os.UserHomeDir()
	certsDir := filepath.Join(home, ".kaptan", "certs")

	caKeyPEM, err := os.ReadFile(filepath.Join(certsDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("read CA key: %w", err)
	}
	caCertPEM, err := os.ReadFile(filepath.Join(certsDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("read CA cert: %w", err)
	}

	caKey, caCert, err := parseCACerts(caKeyPEM, caCertPEM)
	if err != nil {
		return err
	}

	if err := generateClientCert(certsDir, caKey, caCert); err != nil {
		return err
	}

	fmt.Printf("Rotated client cert for server %s\n", certRotateServer)
	fmt.Println("Reconnect to the server to use the new certificate.")
	return nil
}

func generateCA() (*ecdsa.PrivateKey, *x509.Certificate, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kaptan-ca"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}

	return key, cert, der, nil
}

func generateClientCert(certsDir string, caKey *ecdsa.PrivateKey, caCert *x509.Certificate) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "kaptan-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}

	if err := writePEM(filepath.Join(certsDir, "client.key"), "EC PRIVATE KEY", ecKeyBytes(key)); err != nil {
		return err
	}
	return writePEM(filepath.Join(certsDir, "client.crt"), "CERTIFICATE", der)
}

func parseCACerts(keyPEM, certPEM []byte) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("invalid PEM key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}

	block, _ = pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("invalid PEM cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}

	return key, cert, nil
}

func writePEM(path, pemType string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: pemType, Bytes: data})
}

func ecKeyBytes(key *ecdsa.PrivateKey) []byte {
	b, _ := x509.MarshalECPrivateKey(key)
	return b
}
