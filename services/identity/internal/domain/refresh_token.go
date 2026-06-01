package domain

import "time"

type RefreshTokenID string

func (id RefreshTokenID) String() string { return string(id) }

type RefreshToken struct {
	ID         RefreshTokenID
	UserID     UserID
	TokenHash  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *RefreshTokenID
}

func (t *RefreshToken) IsRevoked() bool { return t.RevokedAt != nil }

func (t *RefreshToken) IsExpired(now time.Time) bool { return !now.Before(t.ExpiresAt) }

func (t *RefreshToken) IsActive(now time.Time) bool {
	return !t.IsRevoked() && !t.IsExpired(now)
}
