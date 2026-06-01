// Package registeruser implements the registration use case: validate input,
// hash the password, and persist the user with its outbox event in one tx.
package registeruser

import (
	"context"
	"fmt"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type UseCase struct {
	hasher  ports.PasswordHasher
	encoder ports.EventEncoder
	tx      ports.TxManager
	clock   ports.Clock
	ids     ports.IDGenerator
}

func New(hasher ports.PasswordHasher, encoder ports.EventEncoder, tx ports.TxManager, clock ports.Clock, ids ports.IDGenerator) *UseCase {
	return &UseCase{hasher: hasher, encoder: encoder, tx: tx, clock: clock, ids: ids}
}

func (uc *UseCase) Execute(ctx context.Context, req ports.RegisterUserRequest) (ports.RegisterUserResponse, error) {
	email, err := domain.ParseEmail(req.Email)
	if err != nil {
		return ports.RegisterUserResponse{}, fmt.Errorf("register user: %w", err)
	}

	plain := domain.PlainPassword(req.Password)
	if err := plain.Validate(); err != nil {
		return ports.RegisterUserResponse{}, fmt.Errorf("register user: %w", err)
	}

	hash, err := uc.hasher.Hash(ctx, plain)
	if err != nil {
		return ports.RegisterUserResponse{}, fmt.Errorf("register user: hash password: %w", err)
	}

	now := uc.clock.Now()
	user := domain.NewUser(domain.UserID(uc.ids.NewID()), email, hash, []domain.Role{domain.RoleCustomer}, now)

	// Build the event before the tx so a marshalling failure aborts cheaply; it
	// is then written to the outbox in the same tx as the user row.
	event, err := uc.encoder.UserRegistered(user, now)
	if err != nil {
		return ports.RegisterUserResponse{}, fmt.Errorf("register user: encode event: %w", err)
	}

	if err := uc.tx.WithinTx(ctx, func(ctx context.Context, repos ports.RepoSet) error {
		if err := repos.Users.Insert(ctx, user); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if err := repos.Events.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish event: %w", err)
		}
		return nil
	}); err != nil {
		return ports.RegisterUserResponse{}, fmt.Errorf("register user: %w", err)
	}

	return ports.RegisterUserResponse{UserID: user.ID.String(), Email: user.Email.String()}, nil
}

var _ ports.Registrar = (*UseCase)(nil)
