package loginuser_test

import (
	"context"
	"testing"
	"time"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/loginuser"
	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type fakeUserRepo struct {
	ports.UserRepository
	user *domain.User
	err  error
}

func (f fakeUserRepo) GetByEmail(context.Context, domain.Email) (*domain.User, error) {
	return f.user, f.err
}

type fakeRefreshRepo struct {
	ports.RefreshTokenRepository
	inserted *domain.RefreshToken
}

func (f *fakeRefreshRepo) Insert(_ context.Context, t *domain.RefreshToken) error {
	f.inserted = t
	return nil
}

type fakeHasher struct {
	ports.PasswordHasher
	match bool
	err   error
}

func (f fakeHasher) Verify(context.Context, domain.PlainPassword, domain.PasswordHash) (bool, error) {
	return f.match, f.err
}

type fakeIssuer struct{ token domain.AccessToken }

func (f fakeIssuer) Issue(*domain.User) (domain.AccessToken, error) { return f.token, nil }

type fakeRefreshGen struct{ token, hash string }

func (f fakeRefreshGen) Generate() (token, hash string, err error) { return f.token, f.hash, nil }
func (f fakeRefreshGen) Hash(t string) string                      { return "sha256:" + t }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type stubIDGen struct{}

func (stubIDGen) NewID() string { return "rt-1" }

func validUser() *domain.User {
	return domain.NewUser("u-1", "a@b.c", "stored-hash", nil, time.Now())
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := now.Add(15 * time.Minute)
	tokens := &fakeRefreshRepo{}

	uc := loginuser.New(
		fakeUserRepo{user: validUser()},
		tokens,
		fakeHasher{match: true},
		fakeIssuer{token: domain.AccessToken{Value: "access.jwt", ExpiresAt: exp}},
		fakeRefreshGen{token: "refresh-plain", hash: "refresh-hash"},
		fixedClock{t: now},
		stubIDGen{},
		720*time.Hour,
	)

	resp, err := uc.Execute(context.Background(), ports.LoginUserRequest{Email: "a@b.c", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.AccessToken != "access.jwt" || !resp.AccessTokenExpiresAt.Equal(exp) {
		t.Fatalf("access token/expiry wrong: %+v", resp)
	}
	if resp.RefreshToken != "refresh-plain" {
		t.Fatalf("refresh token = %q, want the plaintext", resp.RefreshToken)
	}
	if tokens.inserted == nil {
		t.Fatal("refresh token row not inserted")
	}
	if tokens.inserted.TokenHash != "refresh-hash" {
		t.Fatalf("stored hash = %q, want refresh-hash (never the plaintext)", tokens.inserted.TokenHash)
	}
	if !tokens.inserted.ExpiresAt.Equal(now.Add(720 * time.Hour)) {
		t.Fatalf("refresh expiry = %v, want now+ttl", tokens.inserted.ExpiresAt)
	}
	if tokens.inserted.UserID != "u-1" {
		t.Fatalf("refresh user id = %q", tokens.inserted.UserID)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		repo   fakeUserRepo
		hasher fakeHasher
	}{
		{"unknown email", fakeUserRepo{err: domain.ErrUserNotFound}, fakeHasher{match: true}},
		{"wrong password", fakeUserRepo{user: validUser()}, fakeHasher{match: false}},
		{"malformed email", fakeUserRepo{user: validUser()}, fakeHasher{match: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := loginuser.New(
				tc.repo, &fakeRefreshRepo{}, tc.hasher,
				fakeIssuer{}, fakeRefreshGen{}, fixedClock{}, stubIDGen{}, time.Hour,
			)

			email := "a@b.c"
			if tc.name == "malformed email" {
				email = "not-an-email"
			}

			_, err := uc.Execute(context.Background(), ports.LoginUserRequest{Email: email, Password: "hunter2!"})

			var e *errors.Error
			if !errors.As(err, &e) || e.Code != "identity.invalid_credentials" {
				t.Fatalf("error = %v, want identity.invalid_credentials", err)
			}
		})
	}
}
