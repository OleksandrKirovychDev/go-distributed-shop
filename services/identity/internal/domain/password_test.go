package domain_test

import (
	"strings"
	"testing"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/domain"
)

func TestPlainPassword_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"too short", "short7!", false},
		{"min length", "hunter2!", true},
		{"long ok", strings.Repeat("a", 200), true},
		{"too long", strings.Repeat("a", 1025), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.PlainPassword(tc.in).Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate(%q) = %v, want nil", tc.name, err)
			}
			if !tc.ok && !errors.IsKind(err, errors.KindInvalid) {
				t.Fatalf("Validate(%q) = %v, want Invalid", tc.name, err)
			}
		})
	}
}

func TestPlainPassword_StringRedacts(t *testing.T) {
	t.Parallel()
	if got := domain.PlainPassword("super-secret").String(); strings.Contains(got, "secret") {
		t.Fatalf("String() leaked the password: %q", got)
	}
}
