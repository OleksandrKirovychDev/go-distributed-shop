package auth

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/online-shop/pkg/errors"
)

type TokenIssuer struct {
	key      *rsa.PrivateKey
	kid      string
	issuer   string
	audience string
	ttl      time.Duration
}

func NewTokenIssuer(key *rsa.PrivateKey, issuer, audience string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{key: key, kid: KeyID(&key.PublicKey), issuer: issuer, audience: audience, ttl: ttl}
}

// Issue signs an RS256 access token and returns it with its expiry instant.
func (i *TokenIssuer) Issue(userID, email string, roles []string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(i.ttl)

	claims := Claims{
		Email: email,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    i.issuer,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Stamp the kid so a multi-key JWKS verifier can select the right key; with a
	// single published key it is informational but keeps the door open to rotation.
	token.Header["kid"] = i.kid

	signed, err := token.SignedString(i.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, expiresAt, nil
}

type Verifier struct {
	pub      *rsa.PublicKey
	issuer   string
	audience string
}

func NewVerifier(pub *rsa.PublicKey, issuer, audience string) *Verifier {
	return &Verifier{pub: pub, issuer: issuer, audience: audience}
}

// Verify parses and validates a token, returning its claims. WithValidMethods
// pins RS256 so a forged token cannot downgrade the algorithm (e.g. to "none"
// or HMAC keyed on the public key).
func (v *Verifier) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return v.pub, nil },
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, errors.NewUnauthorized("auth.invalid_token", "invalid or expired token", err)
	}

	claims.UserID = claims.Subject
	return claims, nil
}
