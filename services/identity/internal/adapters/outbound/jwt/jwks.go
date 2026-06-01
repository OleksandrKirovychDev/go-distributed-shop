package jwt

import (
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/online-shop/pkg/auth"
)

// NewJWKSHandler serves the signing key's public half as an RFC 7517 JWK Set,
// the document the Apollo Router fetches to validate the RS256 access tokens
// this service issues. The set is rendered once here because the key is fixed
// for the process lifetime, so each request is a header write plus a buffer copy
// with no per-request marshalling.
func NewJWKSHandler(pub *rsa.PublicKey) (http.Handler, error) {
	body, err := auth.JWKS(pub)
	if err != nil {
		return nil, fmt.Errorf("jwt: render jwks: %w", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The key is stable for the process, so verifiers may cache the set; keep
		// the window short enough that a redeploy with a new key is picked up soon.
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(body)
	}), nil
}
