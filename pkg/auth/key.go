package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// GenerateEphemeralKey mints a 2048-bit RSA key for the process lifetime. Its
// kid changes on every restart, so it is only sound for single-process dev runs
// where signer and verifier are the same process; a multi-replica or
// JWKS-published deployment wants LoadPrivateKeyFromPEM for a stable kid.
func GenerateEphemeralKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("auth: generate key: %w", err)
	}
	return key, nil
}

// LoadPrivateKeyFromPEM parses a PEM-encoded RSA private key, accepting both
// PKCS#1 ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") so the caller need not
// know which form their keygen produced.
func LoadPrivateKeyFromPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("auth: no PEM block found in key data")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("auth: private key is %T, want *rsa.PrivateKey", parsed)
	}
	return key, nil
}
