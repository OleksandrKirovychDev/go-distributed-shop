package errors_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/online-shop/pkg/errors"
)

func TestConstructors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  *errors.Error
		kind errors.Kind
	}{
		{"invalid", errors.NewInvalid("c", "m", nil), errors.KindInvalid},
		{"not_found", errors.NewNotFound("c", "m", nil), errors.KindNotFound},
		{"conflict", errors.NewConflict("c", "m", nil), errors.KindConflict},
		{"unauthorized", errors.NewUnauthorized("c", "m", nil), errors.KindUnauthorized},
		{"forbidden", errors.NewForbidden("c", "m", nil), errors.KindForbidden},
		{"internal", errors.NewInternal("c", "m", nil), errors.KindInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got.Kind != tc.kind {
				t.Fatalf("kind: got %v want %v", tc.got.Kind, tc.kind)
			}
			if tc.got.Code != "c" || tc.got.Message != "m" {
				t.Fatalf("fields not propagated: %+v", tc.got)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	cases := map[errors.Kind]string{
		errors.KindUnknown:      "unknown",
		errors.KindInvalid:      "invalid",
		errors.KindNotFound:     "not_found",
		errors.KindConflict:     "conflict",
		errors.KindUnauthorized: "unauthorized",
		errors.KindForbidden:    "forbidden",
		errors.KindInternal:     "internal",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestErrorFormatting(t *testing.T) {
	t.Parallel()

	inner := stderrors.New("inner")
	cases := []struct {
		name string
		err  *errors.Error
		want string
	}{
		{"kind only", &errors.Error{Kind: errors.KindNotFound}, "not_found"},
		{"kind+msg", errors.NewInvalid("", "bad email", nil), "invalid: bad email"},
		{"kind+wrap", &errors.Error{Kind: errors.KindInternal, Wrapped: inner}, "internal: inner"},
		{"kind+msg+wrap", errors.NewInternal("", "db down", inner), "internal: db down: inner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	t.Parallel()

	sentinel := stderrors.New("boom")
	wrapped := errors.NewInternal("", "wrap", sentinel)

	if !stderrors.Is(wrapped, sentinel) {
		t.Fatal("errors.Is should traverse Unwrap to the sentinel")
	}
	if got := stderrors.Unwrap(wrapped); got != sentinel {
		t.Fatalf("Unwrap() = %v, want %v", got, sentinel)
	}
}

func TestIsBetweenEnvelopes(t *testing.T) {
	t.Parallel()

	a := errors.NewNotFound("a", "msg-a", nil)
	b := errors.NewNotFound("b", "msg-b", nil)
	c := errors.NewConflict("c", "msg-c", nil)

	if !stderrors.Is(a, b) {
		t.Fatal("two envelopes with the same Kind should match via errors.Is")
	}
	if stderrors.Is(a, c) {
		t.Fatal("envelopes with different Kinds should not match")
	}
}

func TestIsKind(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("layer above: %w", errors.NewConflict("dup", "duplicate", nil))

	if !errors.IsKind(err, errors.KindConflict) {
		t.Fatal("IsKind should traverse the chain")
	}
	if errors.IsKind(err, errors.KindInvalid) {
		t.Fatal("IsKind should reject mismatched kinds")
	}
	if errors.IsKind(stderrors.New("plain"), errors.KindInternal) {
		t.Fatal("IsKind should return false for non-envelope errors")
	}
}

func TestAs(t *testing.T) {
	t.Parallel()

	original := errors.NewForbidden("rbac.deny", "no perms", nil)
	wrapped := fmt.Errorf("middleware: %w", original)

	var target *errors.Error
	if !errors.As(wrapped, &target) {
		t.Fatal("As should find the envelope through the chain")
	}
	if target.Code != "rbac.deny" || target.Kind != errors.KindForbidden {
		t.Fatalf("As populated wrong envelope: %+v", target)
	}
}

func TestNilReceiverSafe(t *testing.T) {
	t.Parallel()

	var e *errors.Error
	if got := e.Error(); got != "<nil>" {
		t.Fatalf("nil receiver Error() = %q, want %q", got, "<nil>")
	}
	if e.Unwrap() != nil {
		t.Fatal("nil receiver Unwrap should be nil")
	}
	if e.Is(stderrors.New("x")) {
		t.Fatal("nil receiver Is should be false")
	}
}

func TestTypedNilTargetSafe(t *testing.T) {
	t.Parallel()

	// A typed-nil *Error wrapped in error is NOT interface-nil; errors.AsType
	// returns (nil, true) for it. Both Is and IsKind must guard against the
	// resulting nil deref instead of panicking.
	var typedNil *errors.Error
	concrete := errors.NewNotFound("c", "m", nil)

	if stderrors.Is(concrete, typedNil) {
		t.Fatal("Is(concrete, typed-nil) should be false, not a match")
	}
	if errors.IsKind(typedNil, errors.KindNotFound) {
		t.Fatal("IsKind(typed-nil, ...) should be false")
	}
}
