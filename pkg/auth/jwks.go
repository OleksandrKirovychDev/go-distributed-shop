package auth

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

// KeyID derives a stable key ID from the public key as the RFC 7638 JWK
// thumbprint: SHA-256 over the canonical {e,kty,n} JWK (members in lexicographic
// order, no whitespace), base64url-encoded. Being a pure function of the key
// material, the issuer's `kid` header and the JWKS `kid` agree without any
// shared configuration.
func KeyID(pub *rsa.PublicKey) string {
	// json.Marshal of a struct emits fields in declaration order with no extra
	// whitespace, which is exactly the canonical form RFC 7638 requires.
	canonical, _ := json.Marshal(struct {
		E   string `json:"e"`
		Kty string `json:"kty"`
		N   string `json:"n"`
	}{E: encodeBase64URLUint(big.NewInt(int64(pub.E))), Kty: "RSA", N: encodeBase64URLUint(pub.N)})

	sum := sha256.Sum256(canonical)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// JWKS renders the public key as a single-key RFC 7517 JWK Set the router (or
// any verifier) can fetch to validate RS256 tokens this key signs.
func JWKS(pub *rsa.PublicKey) ([]byte, error) {
	type jwk struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}

	doc := struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: KeyID(pub),
		N:   encodeBase64URLUint(pub.N),
		E:   encodeBase64URLUint(big.NewInt(int64(pub.E))),
	}}}

	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("auth: marshal jwks: %w", err)
	}
	return out, nil
}

// encodeBase64URLUint encodes a non-negative big integer as the big-endian,
// minimal-length base64url string JWK uses for modulus and exponent (RFC 7518
// §2, "Base64urlUInt").
func encodeBase64URLUint(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}
