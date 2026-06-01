package domain_test

import (
	"testing"
	"time"

	"github.com/online-shop/services/identity/internal/domain"
)

func TestRefreshToken_State(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Hour)

	cases := []struct {
		name    string
		token   domain.RefreshToken
		expired bool
		revoked bool
		active  bool
	}{
		{
			name:   "fresh",
			token:  domain.RefreshToken{ExpiresAt: now.Add(time.Hour)},
			active: true,
		},
		{
			name:    "expired",
			token:   domain.RefreshToken{ExpiresAt: now.Add(-time.Minute)},
			expired: true,
		},
		{
			name:    "expired exactly now",
			token:   domain.RefreshToken{ExpiresAt: now},
			expired: true,
		},
		{
			name:    "revoked",
			token:   domain.RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt},
			revoked: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.token.IsExpired(now); got != tc.expired {
				t.Errorf("IsExpired = %v, want %v", got, tc.expired)
			}
			if got := tc.token.IsRevoked(); got != tc.revoked {
				t.Errorf("IsRevoked = %v, want %v", got, tc.revoked)
			}
			if got := tc.token.IsActive(now); got != tc.active {
				t.Errorf("IsActive = %v, want %v", got, tc.active)
			}
		})
	}
}
