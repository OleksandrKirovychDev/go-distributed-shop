package postgres

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/online-shop/pkg/errors"
	pkgpg "github.com/online-shop/pkg/postgres"

	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres/gen"
	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type RefreshTokenRepository struct {
	q *gen.Queries
}

func NewRefreshTokenRepository(db pkgpg.Querier) *RefreshTokenRepository {
	return &RefreshTokenRepository{q: gen.New(db)}
}

func (r *RefreshTokenRepository) Insert(ctx context.Context, t *domain.RefreshToken) error {
	id, err := uuid.Parse(t.ID.String())
	if err != nil {
		return errors.NewInternal("identity.bad_refresh_token_id", "refresh token id is not a uuid", err)
	}
	userID, err := uuid.Parse(t.UserID.String())
	if err != nil {
		return errors.NewInternal("identity.bad_user_id", "user id is not a uuid", err)
	}

	if err := r.q.InsertRefreshToken(ctx, gen.InsertRefreshTokenParams{
		ID:        id,
		UserID:    userID,
		TokenHash: t.TokenHash,
		IssuedAt:  t.IssuedAt,
		ExpiresAt: t.ExpiresAt,
	}); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, errors.NewNotFound("identity.refresh_token_not_found", "refresh token not found", nil)
		}
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return toDomainRefreshToken(row), nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id domain.RefreshTokenID, now time.Time, replacedBy *domain.RefreshTokenID) error {
	tokenID, err := uuid.Parse(id.String())
	if err != nil {
		return errors.NewInternal("identity.bad_refresh_token_id", "refresh token id is not a uuid", err)
	}

	var replaced *uuid.UUID
	if replacedBy != nil {
		parsed, err := uuid.Parse(replacedBy.String())
		if err != nil {
			return errors.NewInternal("identity.bad_refresh_token_id", "replacement id is not a uuid", err)
		}
		replaced = &parsed
	}

	if err := r.q.RevokeRefreshToken(ctx, gen.RevokeRefreshTokenParams{
		ID:         tokenID,
		RevokedAt:  &now,
		ReplacedBy: replaced,
	}); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, id domain.UserID, now time.Time) error {
	userID, err := uuid.Parse(id.String())
	if err != nil {
		return errors.NewInternal("identity.bad_user_id", "user id is not a uuid", err)
	}
	if err := r.q.RevokeAllRefreshTokensForUser(ctx, gen.RevokeAllRefreshTokensForUserParams{
		UserID:    userID,
		RevokedAt: &now,
	}); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return nil
}

var _ ports.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
