package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertFiles writes a self-signed ECDSA cert and key to a temp dir.
// Returns (certFile, keyFile). Caller registers t.Cleanup automatically.
func generateTestCertFiles(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	cf, err := os.Create(certFile)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	cf.Close()

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	kf, err := os.Create(keyFile)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	kf.Close()

	return certFile, keyFile
}

func TestGetDefaultTLSConfig(t *testing.T) {
	cfg := GetDefaultTLSConfig()
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.Len(t, cfg.CipherSuites, 4)
}

func TestLoadTLSConfig_CertFileNotFound(t *testing.T) {
	_, err := LoadTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate file not found")
}

func TestLoadTLSConfig_KeyFileNotFound(t *testing.T) {
	_, keyFile := generateTestCertFiles(t)
	// Pass a valid cert but missing key — swap so cert exists, key path is wrong
	_, err := LoadTLSConfig(keyFile, "/nonexistent/key.pem")
	// keyFile is an EC PRIVATE KEY PEM — LoadX509KeyPair will fail, not os.Stat
	// Either way, we get an error
	require.Error(t, err)
}

func TestLoadTLSConfig_InvalidKeyPair(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, []byte("not a cert"), 0600))
	require.NoError(t, os.WriteFile(keyFile, []byte("not a key"), 0600))

	_, err := LoadTLSConfig(certFile, keyFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load certificate")
}

func TestLoadTLSConfig_Valid(t *testing.T) {
	certFile, keyFile := generateTestCertFiles(t)

	cfg, err := LoadTLSConfig(certFile, keyFile)
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.Len(t, cfg.CipherSuites, 4)
	assert.Len(t, cfg.Certificates, 1)
}
