package auth

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	// UserID mirrors RegisteredClaims.Subject; json:"-" keeps the user ID on the
	// wire only as `sub` rather than duplicating it as a custom claim.
	UserID string   `json:"-"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}
