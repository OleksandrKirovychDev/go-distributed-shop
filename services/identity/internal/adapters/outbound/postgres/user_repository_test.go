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

func TestUserRepository_RoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepository(testPool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user := domain.NewUser(domain.UserID(uuid.NewString()), "roundtrip@example.com", "argon-hash",
		[]domain.Role{domain.RoleCustomer}, now)
	if err := repo.Insert(ctx, user); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	byEmail, err := repo.GetByEmail(ctx, "roundtrip@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if byEmail.ID != user.ID || byEmail.PasswordHash != "argon-hash" {
		t.Fatalf("got %+v", byEmail)
	}
	if len(byEmail.Roles) != 1 || byEmail.Roles[0] != domain.RoleCustomer {
		t.Fatalf("roles = %v", byEmail.Roles)
	}
	if !byEmail.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %v, want %v", byEmail.CreatedAt, now)
	}

	byID, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Email != "roundtrip@example.com" {
		t.Fatalf("got %+v", byID)
	}
}

func TestUserRepository_DuplicateEmailIsConflict(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepository(testPool)

	a := domain.NewUser(domain.UserID(uuid.NewString()), "dup@example.com", "h", nil, time.Now())
	b := domain.NewUser(domain.UserID(uuid.NewString()), "dup@example.com", "h", nil, time.Now())
	if err := repo.Insert(ctx, a); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := repo.Insert(ctx, b); !errors.IsKind(err, errors.KindConflict) {
		t.Fatalf("duplicate insert error = %v, want Conflict", err)
	}
}

func TestUserRepository_GetByIDNotFound(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	if _, err := repo.GetByID(context.Background(), domain.UserID(uuid.NewString())); !errors.IsKind(err, errors.KindNotFound) {
		t.Fatalf("error = %v, want NotFound", err)
	}
}
