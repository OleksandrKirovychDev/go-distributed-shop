package getuser_test

import (
	"context"
	"testing"
	"time"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/getuser"
	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type fakeUserRepo struct {
	ports.UserRepository
	user *domain.User
	err  error
}

func (f fakeUserRepo) GetByID(context.Context, domain.UserID) (*domain.User, error) {
	return f.user, f.err
}

func TestGetUser_Found(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	uc := getuser.New(fakeUserRepo{user: &domain.User{
		ID:        "u-1",
		Email:     "a@b.c",
		Roles:     []domain.Role{domain.RoleCustomer},
		CreatedAt: created,
	}})

	resp, err := uc.Execute(context.Background(), ports.GetUserRequest{UserID: "u-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.User.ID != "u-1" || resp.User.Email != "a@b.c" {
		t.Fatalf("user view = %+v", resp.User)
	}
	if len(resp.User.Roles) != 1 || resp.User.Roles[0] != "customer" {
		t.Fatalf("roles = %v", resp.User.Roles)
	}
	if !resp.User.CreatedAt.Equal(created) {
		t.Fatalf("created_at = %v", resp.User.CreatedAt)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()

	uc := getuser.New(fakeUserRepo{err: domain.ErrUserNotFound})

	_, err := uc.Execute(context.Background(), ports.GetUserRequest{UserID: "missing"})
	if !errors.IsKind(err, errors.KindNotFound) {
		t.Fatalf("error = %v, want NotFound", err)
	}
}
