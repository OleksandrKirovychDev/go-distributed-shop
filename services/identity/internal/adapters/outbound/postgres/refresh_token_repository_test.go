//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres"
	"github.com/online-shop/services/identity/internal/domain"
)

func TestRefreshTokenRepository_RoundTripAndRotate(t *testing.T) {
	ctx := context.Background()
	users := postgres.NewUserRepository(testPool)
	tokens := postgres.NewRefreshTokenRepository(testPool)

	user := domain.NewUser(domain.UserID(uuid.NewString()), "rt@example.com", "h", nil, time.Now())
	if err := users.Insert(ctx, user); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	current := &domain.RefreshToken{
		ID:        domain.RefreshTokenID(uuid.NewString()),
		UserID:    user.ID,
		TokenHash: "hash-1",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := tokens.Insert(ctx, current); err != nil {
		t.Fatalf("insert token: %v", err)
	}

	got, err := tokens.GetByHash(ctx, "hash-1")
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != current.ID || got.UserID != user.ID || !got.IsActive(now) {
		t.Fatalf("got %+v", got)
	}

	// Rotate: insert replacement, revoke the old pointing at the new.
	next := &domain.RefreshToken{
		ID:        domain.RefreshTokenID(uuid.NewString()),
		UserID:    user.ID,
		TokenHash: "hash-2",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := tokens.Insert(ctx, next); err != nil {
		t.Fatalf("insert replacement: %v", err)
	}
	if err := tokens.Revoke(ctx, current.ID, now, &next.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	revoked, err := tokens.GetByHash(ctx, "hash-1")
	if err != nil {
		t.Fatalf("GetByHash after revoke: %v", err)
	}
	if !revoked.IsRevoked() {
		t.Fatal("token should be revoked")
	}
	if revoked.ReplacedBy == nil || *revoked.ReplacedBy != next.ID {
		t.Fatalf("replaced_by = %v, want %v", revoked.ReplacedBy, next.ID)
	}
}

func TestRefreshTokenRepository_GetByHashNotFound(t *testing.T) {
	tokens := postgres.NewRefreshTokenRepository(testPool)
	if _, err := tokens.GetByHash(context.Background(), "missing-hash"); !errors.IsKind(err, errors.KindNotFound) {
		t.Fatalf("error = %v, want NotFound", err)
	}
}
