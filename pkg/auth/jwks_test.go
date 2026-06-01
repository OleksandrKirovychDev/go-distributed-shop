package auth_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/online-shop/pkg/auth"
)

func TestKeyID_StableAcrossEncodings(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	kid := auth.KeyID(&key.PublicKey)

	if again := auth.KeyID(&key.PublicKey); again != kid {
		t.Fatalf("KeyID not deterministic: %q vs %q", kid, again)
	}

	// A key round-tripped through PEM is the same key, so the thumbprint must not
	// change — this is the property that gives a durable signer a stable kid.
	for _, pemBytes := range [][]byte{pkcs1PEM(t, key), pkcs8PEM(t, key)} {
		loaded, err := auth.LoadPrivateKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("LoadPrivateKeyFromPEM: %v", err)
		}
		if got := auth.KeyID(&loaded.PublicKey); got != kid {
			t.Fatalf("kid changed across PEM round-trip: %q != %q", got, kid)
		}
	}
}

func TestKeyID_DistinctKeysDiffer(t *testing.T) {
	t.Parallel()

	if a, b := auth.KeyID(&newKey(t).PublicKey), auth.KeyID(&newKey(t).PublicKey); a == b {
		t.Fatalf("distinct keys produced the same kid: %q", a)
	}
}

func TestJWKS_StructureAndKeyMaterial(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	raw, err := auth.JWKS(&key.PublicKey)
	if err != nil {
		t.Fatalf("JWKS: %v", err)
	}

	var doc struct {
		Keys []struct {
			Kty, Use, Alg, Kid, N, E string
		} `json:"keys"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal jwks: %v", err)
	}
	if len(doc.Keys) != 1 {
		t.Fatalf("want exactly one key, got %d", len(doc.Keys))
	}

	k := doc.Keys[0]
	if k.Kty != "RSA" || k.Use != "sig" || k.Alg != "RS256" {
		t.Fatalf("unexpected JWK header fields: %+v", k)
	}
	if k.Kid != auth.KeyID(&key.PublicKey) {
		t.Fatalf("JWKS kid %q disagrees with KeyID", k.Kid)
	}

	// n and e must decode back to the exact public key the verifier will trust.
	n := new(big.Int).SetBytes(decodeB64URL(t, k.N))
	e := new(big.Int).SetBytes(decodeB64URL(t, k.E))
	if n.Cmp(key.N) != 0 {
		t.Fatalf("modulus mismatch")
	}
	if e.Int64() != int64(key.E) {
		t.Fatalf("exponent = %d, want %d", e.Int64(), key.E)
	}
}

func TestLoadPrivateKeyFromPEM_RejectsGarbage(t *testing.T) {
	t.Parallel()

	if _, err := auth.LoadPrivateKeyFromPEM([]byte("not a pem")); err == nil {
		t.Fatal("expected error for non-PEM input")
	}
}

func pkcs1PEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func pkcs8PEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func decodeB64URL(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode base64url %q: %v", s, err)
	}
	return b
}
