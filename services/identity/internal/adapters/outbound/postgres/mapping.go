// Package postgres adapts the sqlc-generated queries to the app's outbound
// repository ports. Generated rows and pgx types never escape this package:
// every method takes and returns domain types. Repositories are parameterised
// on a Querier, so the same code runs against the pool (autocommit) or a
// pgx.Tx (inside TxManager.WithinTx).
package postgres

import (
	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres/gen"
	"github.com/online-shop/services/identity/internal/domain"
)

func toDomainUser(row gen.User) (*domain.User, error) {
	roles, err := toDomainRoles(row.Roles)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		ID:           domain.UserID(row.ID.String()),
		Email:        domain.Email(row.Email),
		PasswordHash: domain.PasswordHash(row.PasswordHash),
		Roles:        roles,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

// toDomainRoles validates stored roles. A value the domain doesn't recognise is
// data corruption (we only ever write valid roles), hence Internal, not Invalid.
func toDomainRoles(raw []string) ([]domain.Role, error) {
	roles := make([]domain.Role, len(raw))
	for i, r := range raw {
		role, err := domain.ParseRole(r)
		if err != nil {
			return nil, errors.NewInternal("identity.corrupt_role", "stored role is not recognised", err)
		}
		roles[i] = role
	}
	return roles, nil
}

func rolesToStrings(roles []domain.Role) []string {
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = r.String()
	}
	return out
}

func toDomainRefreshToken(row gen.RefreshToken) *domain.RefreshToken {
	rt := &domain.RefreshToken{
		ID:        domain.RefreshTokenID(row.ID.String()),
		UserID:    domain.UserID(row.UserID.String()),
		TokenHash: row.TokenHash,
		IssuedAt:  row.IssuedAt,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt,
	}
	if row.ReplacedBy != nil {
		replaced := domain.RefreshTokenID(row.ReplacedBy.String())
		rt.ReplacedBy = &replaced
	}
	return rt
}
