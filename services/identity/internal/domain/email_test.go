package domain_test

import (
	"strings"
	"testing"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/domain"
)

func TestParseEmail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want domain.Email
		ok   bool
	}{
		{"plain", "a@b.c", "a@b.c", true},
		{"uppercased", "Alice@Example.COM", "alice@example.com", true},
		{"surrounding space", "  bob@example.com  ", "bob@example.com", true},
		{"empty", "", "", false},
		{"blank", "   ", "", false},
		{"no at", "not-an-email", "", false},
		{"display name form", "Foo <foo@example.com>", "", false},
		{"too long", strings.Repeat("a", 250) + "@b.co", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseEmail(tc.in)
			if tc.ok {
				if err != nil {
					t.Fatalf("ParseEmail(%q) error: %v", tc.in, err)
				}
				if got != tc.want {
					t.Fatalf("ParseEmail(%q) = %q, want %q", tc.in, got, tc.want)
				}
				return
			}
			if !errors.IsKind(err, errors.KindInvalid) {
				t.Fatalf("ParseEmail(%q) = (%q, %v), want Invalid error", tc.in, got, err)
			}
		})
	}
}
