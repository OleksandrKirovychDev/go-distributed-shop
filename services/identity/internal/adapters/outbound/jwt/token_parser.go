package jwt

import (
	"fmt"

	"github.com/online-shop/pkg/auth"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type TokenParser struct {
	verifier *auth.Verifier
}

func NewTokenParser(verifier *auth.Verifier) *TokenParser {
	return &TokenParser{verifier: verifier}
}

func (p *TokenParser) Parse(raw string) (domain.Claims, error) {
	claims, err := p.verifier.Verify(raw)
	if err != nil {
		return domain.Claims{}, fmt.Errorf("jwt: verify access token: %w", err)
	}

	roles := make([]domain.Role, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		role, parseErr := domain.ParseRole(r)
		if parseErr != nil {
			return domain.Claims{}, domain.ErrTokenInvalid
		}
		roles = append(roles, role)
	}

	return domain.Claims{
		UserID: domain.UserID(claims.UserID),
		Email:  domain.Email(claims.Email),
		Roles:  roles,
	}, nil
}

var _ ports.TokenParser = (*TokenParser)(nil)
