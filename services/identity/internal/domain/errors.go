package domain

import "github.com/online-shop/pkg/errors"

// Sentinels carry a pkg/errors.Kind so the gRPC adapter maps them to status
// codes without string-sniffing. ErrInvalidCredentials is deliberately shared
// by unknown-email and wrong-password to defeat account enumeration.
var (
	ErrInvalidEmail = errors.NewInvalid("identity.invalid_email", "email is not valid", nil)
	ErrWeakPassword = errors.NewInvalid("identity.weak_password", "password does not meet the policy", nil)
	ErrInvalidRole  = errors.NewInvalid("identity.invalid_role", "unknown role", nil)

	ErrEmailTaken = errors.NewConflict("identity.email_taken", "email already registered", nil)

	ErrInvalidCredentials = errors.NewUnauthorized("identity.invalid_credentials", "invalid email or password", nil)
	ErrTokenExpired       = errors.NewUnauthorized("identity.token_expired", "refresh token has expired", nil)
	ErrTokenRevoked       = errors.NewUnauthorized("identity.token_revoked", "refresh token has been revoked", nil)
	ErrTokenInvalid       = errors.NewUnauthorized("identity.token_invalid", "token is invalid", nil)

	ErrUserNotFound = errors.NewNotFound("identity.user_not_found", "user not found", nil)
)
