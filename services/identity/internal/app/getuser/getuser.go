// Package getuser implements the read-only user lookup use case.
package getuser

import (
	"context"
	"fmt"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type UseCase struct {
	users ports.UserRepository
}

func New(users ports.UserRepository) *UseCase {
	return &UseCase{users: users}
}

// Execute returns the user by ID. Authorization (self-or-admin) is enforced by
// the inbound adapter from the caller's claims, not here.
func (uc *UseCase) Execute(ctx context.Context, req ports.GetUserRequest) (ports.GetUserResponse, error) {
	user, err := uc.users.GetByID(ctx, domain.UserID(req.UserID))
	if err != nil {
		return ports.GetUserResponse{}, fmt.Errorf("get user: %w", err)
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = r.String()
	}

	return ports.GetUserResponse{User: ports.UserView{
		ID:        user.ID.String(),
		Email:     user.Email.String(),
		Roles:     roles,
		CreatedAt: user.CreatedAt,
	}}, nil
}

var _ ports.UserGetter = (*UseCase)(nil)
