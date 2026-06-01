// Package refreshtoken implements refresh-token rotation: validate the presented
// token, then revoke it and issue a replacement in one transaction.
package refreshtoken

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
	issuer     ports.TokenIssuer
	refresh    ports.RefreshTokenGenerator
	tx         ports.TxManager
	clock      ports.Clock
	ids        ports.IDGenerator
	refreshTTL time.Duration
}

func New(
	users ports.UserRepository,
	tokens ports.RefreshTokenRepository,
	issuer ports.TokenIssuer,
	refresh ports.RefreshTokenGenerator,
	tx ports.TxManager,
	clock ports.Clock,
	ids ports.IDGenerator,
	refreshTTL time.Duration,
) *UseCase {
	return &UseCase{
		users: users, tokens: tokens, issuer: issuer, refresh: refresh,
		tx: tx, clock: clock, ids: ids, refreshTTL: refreshTTL,
	}
}

func (uc *UseCase) Execute(ctx context.Context, req ports.RefreshTokenRequest) (ports.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return ports.RefreshTokenResponse{}, domain.ErrTokenInvalid
	}

	current, err := uc.tokens.GetByHash(ctx, uc.refresh.Hash(req.RefreshToken))
	if err != nil {
		if errors.IsKind(err, errors.KindNotFound) {
			return ports.RefreshTokenResponse{}, domain.ErrTokenInvalid
		}
		return ports.RefreshTokenResponse{}, fmt.Errorf("refresh token: lookup: %w", err)
	}

	now := uc.clock.Now()
	// Revoked is checked before expired: replaying a revoked token is the
	// stronger signal (possible theft), worth surfacing distinctly.
	if current.IsRevoked() {
		return ports.RefreshTokenResponse{}, domain.ErrTokenRevoked
	}
	if current.IsExpired(now) {
		return ports.RefreshTokenResponse{}, domain.ErrTokenExpired
	}

	user, err := uc.users.GetByID(ctx, current.UserID)
	if err != nil {
		return ports.RefreshTokenResponse{}, fmt.Errorf("refresh token: load user: %w", err)
	}

	// Issue before mutating: a sign failure must not leave a rotated-but-
	// unreturned token pair (old revoked, new unknown to the client).
	access, err := uc.issuer.Issue(user)
	if err != nil {
		return ports.RefreshTokenResponse{}, fmt.Errorf("refresh token: issue access token: %w", err)
	}

	plaintext, hash, err := uc.refresh.Generate()
	if err != nil {
		return ports.RefreshTokenResponse{}, fmt.Errorf("refresh token: generate: %w", err)
	}

	next := &domain.RefreshToken{
		ID:        domain.RefreshTokenID(uc.ids.NewID()),
		UserID:    user.ID,
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(uc.refreshTTL),
	}

	if err := uc.tx.WithinTx(ctx, func(ctx context.Context, repos ports.RepoSet) error {
		if err := repos.RefreshTokens.Revoke(ctx, current.ID, now, &next.ID); err != nil {
			return fmt.Errorf("revoke current token: %w", err)
		}
		if err := repos.RefreshTokens.Insert(ctx, next); err != nil {
			return fmt.Errorf("insert rotated token: %w", err)
		}
		return nil
	}); err != nil {
		return ports.RefreshTokenResponse{}, fmt.Errorf("refresh token: rotate: %w", err)
	}

	return ports.RefreshTokenResponse{
		AccessToken:          access.Value,
		RefreshToken:         plaintext,
		AccessTokenExpiresAt: access.ExpiresAt,
	}, nil
}

var _ ports.TokenRefresher = (*UseCase)(nil)
