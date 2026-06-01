package domain_test

import (
	"testing"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/domain"
)

func TestParseRole(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want domain.Role
		ok   bool
	}{
		{"customer", domain.RoleCustomer, true},
		{"admin", domain.RoleAdmin, true},
		{"superuser", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseRole(tc.in)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("ParseRole(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
				}
				return
			}
			if !errors.IsKind(err, errors.KindInvalid) {
				t.Fatalf("ParseRole(%q) = (%q, %v), want Invalid", tc.in, got, err)
			}
		})
	}
}
