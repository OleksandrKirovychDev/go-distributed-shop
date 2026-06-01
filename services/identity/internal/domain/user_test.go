package domain_test

import (
	"testing"
	"time"

	"github.com/online-shop/services/identity/internal/domain"
)

func TestNewUser_DefaultsRoleAndTimestamps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	u := domain.NewUser("u-1", "a@b.c", "hash", nil, now)

	if len(u.Roles) != 1 || u.Roles[0] != domain.RoleCustomer {
		t.Fatalf("roles = %v, want [customer]", u.Roles)
	}
	if !u.CreatedAt.Equal(now) || !u.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps not set to now: created=%v updated=%v", u.CreatedAt, u.UpdatedAt)
	}
}

func TestNewUser_KeepsProvidedRoles(t *testing.T) {
	t.Parallel()

	u := domain.NewUser("u-1", "a@b.c", "hash", []domain.Role{domain.RoleAdmin}, time.Now())
	if len(u.Roles) != 1 || u.Roles[0] != domain.RoleAdmin {
		t.Fatalf("roles = %v, want [admin]", u.Roles)
	}
}

func TestUser_ChangeEmail(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	u := domain.NewUser("u-1", "old@b.c", "hash", nil, created)

	changed := created.Add(time.Hour)
	u.ChangeEmail("new@b.c", changed)

	if u.Email != "new@b.c" {
		t.Fatalf("email = %q, want new@b.c", u.Email)
	}
	if !u.UpdatedAt.Equal(changed) {
		t.Fatalf("updated_at = %v, want %v", u.UpdatedAt, changed)
	}
	if !u.CreatedAt.Equal(created) {
		t.Fatalf("created_at changed: %v", u.CreatedAt)
	}
}
