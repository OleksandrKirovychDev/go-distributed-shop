package verifytoken_test

import (
	"context"
	"testing"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/app/verifytoken"
	"github.com/online-shop/services/identity/internal/domain"
)

type fakeParser struct {
	claims domain.Claims
	err    error
}

func (f fakeParser) Parse(string) (domain.Claims, error) { return f.claims, f.err }

func TestVerify_ValidToken(t *testing.T) {
	t.Parallel()

	uc := verifytoken.New(fakeParser{claims: domain.Claims{
		UserID: "u-1",
		Email:  "a@b.c",
		Roles:  []domain.Role{domain.RoleCustomer, domain.RoleAdmin},
	}})

	resp, err := uc.Execute(context.Background(), ports.VerifyTokenRequest{AccessToken: "good"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !resp.Valid || resp.UserID != "u-1" || resp.Email != "a@b.c" {
		t.Fatalf("response = %+v", resp)
	}
	if len(resp.Roles) != 2 || resp.Roles[0] != "customer" || resp.Roles[1] != "admin" {
		t.Fatalf("roles = %v", resp.Roles)
	}
}

func TestVerify_InvalidTokenIsNotAnError(t *testing.T) {
	t.Parallel()

	uc := verifytoken.New(fakeParser{err: domain.ErrTokenInvalid})

	resp, err := uc.Execute(context.Background(), ports.VerifyTokenRequest{AccessToken: "bad"})
	if err != nil {
		t.Fatalf("invalid token must be a valid:false answer, not an error: %v", err)
	}
	if resp.Valid {
		t.Fatal("response should be valid:false")
	}
}
