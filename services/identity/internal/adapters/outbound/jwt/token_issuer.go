// Package jwt adapts pkg/auth to the app's TokenIssuer and TokenParser ports.
// It is the only seam that imports pkg/auth; app and domain stay JWT-free. The
// ephemeral key wired in Step 4 is replaced by a durable, JWKS-published key in
// Step 5 with no change to these types.
package jwt

import (
	"fmt"

	"github.com/online-shop/pkg/auth"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type TokenIssuer struct {
	issuer *auth.TokenIssuer
}

func NewTokenIssuer(issuer *auth.TokenIssuer) *TokenIssuer {
	return &TokenIssuer{issuer: issuer}
}

func (t *TokenIssuer) Issue(user *domain.User) (domain.AccessToken, error) {
	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = r.String()
	}

	value, expiresAt, err := t.issuer.Issue(user.ID.String(), user.Email.String(), roles)
	if err != nil {
		return domain.AccessToken{}, fmt.Errorf("jwt: issue access token: %w", err)
	}
	return domain.AccessToken{Value: value, ExpiresAt: expiresAt}, nil
}

var _ ports.TokenIssuer = (*TokenIssuer)(nil)
