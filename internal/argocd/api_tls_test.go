package argocd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func emptyListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"items":[]}`)) })
}

func writePEM(t *testing.T, name, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// clientIdentity creates a self-signed client certificate and returns its
// certificate file, key file and parsed certificate.
func clientIdentity(t *testing.T) (certFile, keyFile string, cert *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "argoxd-test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := x509.ParseCertificate(der)
	return writePEM(t, "client.crt", "CERTIFICATE", der), writePEM(t, "client.key", "EC PRIVATE KEY", keyDER), parsed
}

func load(t *testing.T, source *APISource) error {
	t.Helper()
	_, err := source.Load(context.Background())
	return err
}

func TestAPISourceTrustsACustomCA(t *testing.T) {
	server := httptest.NewTLSServer(emptyListHandler())
	defer server.Close()
	caFile := writePEM(t, "ca.crt", "CERTIFICATE", server.Certificate().Raw)

	if err := load(t, NewAPISource(server.URL, "", false)); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("without the CA, Load() error = %v; want a certificate error", err)
	}
	trusting, err := NewConfiguredAPISource(server.URL, "", APIOptions{CACertFile: caFile})
	if err != nil {
		t.Fatalf("NewConfiguredAPISource() error = %v", err)
	}
	if err := load(t, trusting); err != nil {
		t.Fatalf("with the CA, Load() error = %v", err)
	}
}

func TestAPISourceRejectsAnUnusableCAFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.crt")
	if _, err := NewConfiguredAPISource("https://argocd.example.com", "", APIOptions{CACertFile: missing}); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("a missing CA file: error = %v, want one naming %s", err, missing)
	}
	empty := filepath.Join(t.TempDir(), "empty.crt")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfiguredAPISource("https://argocd.example.com", "", APIOptions{CACertFile: empty}); err == nil || !strings.Contains(err.Error(), "no certificates") {
		t.Fatalf("a CA file without certificates: error = %v", err)
	}
}

func TestAPISourcePresentsAClientCertificate(t *testing.T) {
	certFile, keyFile, clientCert := clientIdentity(t)
	pool := x509.NewCertPool()
	pool.AddCert(clientCert)
	server := httptest.NewUnstartedServer(emptyListHandler())
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	caFile := writePEM(t, "ca.crt", "CERTIFICATE", server.Certificate().Raw)

	without, err := NewConfiguredAPISource(server.URL, "", APIOptions{CACertFile: caFile})
	if err != nil {
		t.Fatal(err)
	}
	if err := load(t, without); err == nil {
		t.Fatal("the server accepted a client without a certificate")
	}
	with, err := NewConfiguredAPISource(server.URL, "", APIOptions{CACertFile: caFile, ClientCertFile: certFile, ClientKeyFile: keyFile})
	if err != nil {
		t.Fatalf("NewConfiguredAPISource() error = %v", err)
	}
	if err := load(t, with); err != nil {
		t.Fatalf("with the client certificate, Load() error = %v", err)
	}
}

func TestAPISourceNeedsBothHalvesOfAClientCertificate(t *testing.T) {
	certFile, keyFile, _ := clientIdentity(t)

	if _, err := NewConfiguredAPISource("https://x", "", APIOptions{ClientCertFile: certFile}); err == nil || !strings.Contains(err.Error(), "client key") {
		t.Fatalf("certificate without key: error = %v", err)
	}
	if _, err := NewConfiguredAPISource("https://x", "", APIOptions{ClientKeyFile: keyFile}); err == nil || !strings.Contains(err.Error(), "client certificate") {
		t.Fatalf("key without certificate: error = %v", err)
	}
	if _, err := NewConfiguredAPISource("https://x", "", APIOptions{ClientCertFile: certFile, ClientKeyFile: filepath.Join(t.TempDir(), "missing.key")}); err == nil {
		t.Fatal("a missing key file was accepted")
	}
}

func TestAPISourceGivesUpAfterTheConfiguredTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	source, err := NewConfiguredAPISource(server.URL, "", APIOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	if err := load(t, source); err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("Load() = %v after %v; want a timeout well before the 15s default", err, time.Since(started))
	}
}
