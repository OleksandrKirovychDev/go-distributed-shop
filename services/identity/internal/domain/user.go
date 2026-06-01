// Package domain holds the Identity service's pure business types: users,
// credentials, refresh tokens, and the domain errors adapters translate by
// Kind. It imports only the stdlib and pkg/errors — no ctx, no pgx, no proto,
// and it never reads the clock (callers pass time.Time in).
package domain

import "time"

type UserID string

func (id UserID) String() string { return string(id) }

type User struct {
	ID           UserID
	Email        Email
	PasswordHash PasswordHash
	Roles        []Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser assembles a user from already-validated value objects. The one
// invariant it enforces is "every user has at least one role"; email, hash, and
// id are guaranteed valid by their types and by the caller's id generator.
func NewUser(id UserID, email Email, hash PasswordHash, roles []Role, now time.Time) *User {
	if len(roles) == 0 {
		roles = []Role{RoleCustomer}
	}
	return &User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
		Roles:        roles,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (u *User) ChangeEmail(email Email, now time.Time) {
	u.Email = email
	u.UpdatedAt = now
}
