// Package ports declares the driving (inbound) and driven (outbound) seams of
// the Identity application layer. Request/response DTOs live here too so the
// inbound interfaces and their implementing use cases can both reference them
// without an import cycle. Everything is primitives + domain types — no proto,
// no pgx, no jwt leaks across this boundary.
package ports

import (
	"context"
	"time"
)

type RegisterUserRequest struct {
	Email    string
	Password string
}

type RegisterUserResponse struct {
	UserID string
	Email  string
}

type Registrar interface {
	Execute(ctx context.Context, req RegisterUserRequest) (RegisterUserResponse, error)
}

type LoginUserRequest struct {
	Email    string
	Password string
}

type LoginUserResponse struct {
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt time.Time
}

type Authenticator interface {
	Execute(ctx context.Context, req LoginUserRequest) (LoginUserResponse, error)
}

type RefreshTokenRequest struct {
	RefreshToken string
}

type RefreshTokenResponse struct {
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt time.Time
}

type TokenRefresher interface {
	Execute(ctx context.Context, req RefreshTokenRequest) (RefreshTokenResponse, error)
}

type VerifyTokenRequest struct {
	AccessToken string
}

type VerifyTokenResponse struct {
	Valid  bool
	UserID string
	Email  string
	Roles  []string
}

type TokenVerifier interface {
	Execute(ctx context.Context, req VerifyTokenRequest) (VerifyTokenResponse, error)
}

type GetUserRequest struct {
	UserID string
}

type UserView struct {
	ID        string
	Email     string
	Roles     []string
	CreatedAt time.Time
}

type GetUserResponse struct {
	User UserView
}

type UserGetter interface {
	Execute(ctx context.Context, req GetUserRequest) (GetUserResponse, error)
}
