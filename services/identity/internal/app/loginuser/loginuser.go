// Package loginuser implements the login use case: verify credentials, then
// issue an access token and persist a rotated refresh token.
package loginuser

import (
	"context"
	"fmt"
	"time"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type UseCase struct {
	users      ports.UserRepository
	tokens     ports.RefreshTokenRepository
	hasher     ports.PasswordHasher
	issuer     ports.TokenIssuer
	refresh    ports.RefreshTokenGenerator
	clock      ports.Clock
	ids        ports.IDGenerator
	refreshTTL time.Duration
}

func New(
	users ports.UserRepository,
	tokens ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	issuer ports.TokenIssuer,
	refresh ports.RefreshTokenGenerator,
	clock ports.Clock,
	ids ports.IDGenerator,
	refreshTTL time.Duration,
) *UseCase {
	return &UseCase{
		users: users, tokens: tokens, hasher: hasher, issuer: issuer,
		refresh: refresh, clock: clock, ids: ids, refreshTTL: refreshTTL,
	}
}

func (uc *UseCase) Execute(ctx context.Context, req ports.LoginUserRequest) (ports.LoginUserResponse, error) {
	// A malformed email cannot match a stored user; collapse it into the generic
	// credentials error so login never reveals whether an address exists.
	email, err := domain.ParseEmail(req.Email)
	if err != nil {
		return ports.LoginUserResponse{}, domain.ErrInvalidCredentials
	}

	user, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.IsKind(err, errors.KindNotFound) {
			return ports.LoginUserResponse{}, domain.ErrInvalidCredentials
		}
		return ports.LoginUserResponse{}, fmt.Errorf("login user: get user: %w", err)
	}

	ok, err := uc.hasher.Verify(ctx, domain.PlainPassword(req.Password), user.PasswordHash)
	if err != nil {
		return ports.LoginUserResponse{}, fmt.Errorf("login user: verify password: %w", err)
	}
	if !ok {
		return ports.LoginUserResponse{}, domain.ErrInvalidCredentials
	}

	access, err := uc.issuer.Issue(user)
	if err != nil {
		return ports.LoginUserResponse{}, fmt.Errorf("login user: issue access token: %w", err)
	}

	plaintext, hash, err := uc.refresh.Generate()
	if err != nil {
		return ports.LoginUserResponse{}, fmt.Errorf("login user: generate refresh token: %w", err)
	}

	now := uc.clock.Now()
	rt := &domain.RefreshToken{
		ID:        domain.RefreshTokenID(uc.ids.NewID()),
		UserID:    user.ID,
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(uc.refreshTTL),
	}
	if err := uc.tokens.Insert(ctx, rt); err != nil {
		return ports.LoginUserResponse{}, fmt.Errorf("login user: persist refresh token: %w", err)
	}

	return ports.LoginUserResponse{
		AccessToken:          access.Value,
		RefreshToken:         plaintext,
		AccessTokenExpiresAt: access.ExpiresAt,
	}, nil
}

var _ ports.Authenticator = (*UseCase)(nil)
