package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCertificateFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "key.pem")
	writeSelfSigned(t, certPath, keyPath, "demo.example.com", time.Now().Add(30*24*time.Hour))

	cert, err := parseCertificateFile("demo.example.com", certPath, keyPath, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Domain != "demo.example.com" {
		t.Fatalf("domain: %s", cert.Domain)
	}
	if cert.DaysLeft < 28 || cert.DaysLeft > 31 {
		t.Fatalf("days left: %d", cert.DaysLeft)
	}
	if cert.Source != "test" || !cert.AutoRenew {
		t.Fatalf("meta: %#v", cert)
	}
}

func TestDeleteCertificateRejectsInvalidDomain(t *testing.T) {
	if _, err := deleteCertificate(context.Background(), "../etc", "anpanel"); err == nil {
		t.Fatal("unsafe certificate domain accepted")
	}
}

func writeSelfSigned(t *testing.T, certPath, keyPath, cn string, notAfter time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"AnPanel Test"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		DNSNames:     []string{cn},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		t.Fatal(err)
	}
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), 0600); err != nil {
		t.Fatal(err)
	}
}
