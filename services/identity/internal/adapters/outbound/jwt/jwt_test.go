package jwt_test

import (
	"testing"
	"time"

	"github.com/online-shop/pkg/auth"
	"github.com/online-shop/pkg/errors"

	jwtadapter "github.com/online-shop/services/identity/internal/adapters/outbound/jwt"
	"github.com/online-shop/services/identity/internal/domain"
)

const (
	issuer   = "identity"
	audience = "online-shop"
)

func newAdapters(t *testing.T, ttl time.Duration) (*jwtadapter.TokenIssuer, *jwtadapter.TokenParser) {
	t.Helper()
	key, err := auth.GenerateEphemeralKey()
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return jwtadapter.NewTokenIssuer(auth.NewTokenIssuer(key, issuer, audience, ttl)),
		jwtadapter.NewTokenParser(auth.NewVerifier(&key.PublicKey, issuer, audience))
}

func TestIssueParse_RoundTrip(t *testing.T) {
	t.Parallel()

	issuerAdapter, parser := newAdapters(t, 15*time.Minute)
	user := &domain.User{
		ID:    "u-1",
		Email: "a@b.c",
		Roles: []domain.Role{domain.RoleCustomer, domain.RoleAdmin},
	}

	access, err := issuerAdapter.Issue(user)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if access.Value == "" || access.ExpiresAt.IsZero() {
		t.Fatalf("access token incomplete: %+v", access)
	}

	claims, err := parser.Parse(access.Value)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != "u-1" || claims.Email != "a@b.c" {
		t.Fatalf("claims = %+v", claims)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != domain.RoleCustomer || claims.Roles[1] != domain.RoleAdmin {
		t.Fatalf("roles = %v", claims.Roles)
	}
}

func TestParse_RejectsGarbage(t *testing.T) {
	t.Parallel()

	_, parser := newAdapters(t, 15*time.Minute)
	if _, err := parser.Parse("not.a.jwt"); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("error = %v, want Unauthorized", err)
	}
}

func TestParse_RejectsExpired(t *testing.T) {
	t.Parallel()

	issuerAdapter, parser := newAdapters(t, -time.Minute)
	access, err := issuerAdapter.Issue(&domain.User{ID: "u-1", Email: "a@b.c", Roles: []domain.Role{domain.RoleCustomer}})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := parser.Parse(access.Value); !errors.IsKind(err, errors.KindUnauthorized) {
		t.Fatalf("error = %v, want Unauthorized for expired", err)
	}
}
