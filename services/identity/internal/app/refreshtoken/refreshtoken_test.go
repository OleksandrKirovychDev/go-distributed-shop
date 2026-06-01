package refreshtoken_test

import (
	"context"
	"testing"
	"time"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/app/refreshtoken"
	"github.com/online-shop/services/identity/internal/domain"
)

type revokeCall struct {
	id         domain.RefreshTokenID
	replacedBy *domain.RefreshTokenID
}

type fakeRefreshRepo struct {
	stored   *domain.RefreshToken
	getErr   error
	revoked  []revokeCall
	inserted []*domain.RefreshToken
}

func (f *fakeRefreshRepo) GetByHash(context.Context, string) (*domain.RefreshToken, error) {
	return f.stored, f.getErr
}

func (f *fakeRefreshRepo) Revoke(_ context.Context, id domain.RefreshTokenID, _ time.Time, replacedBy *domain.RefreshTokenID) error {
	f.revoked = append(f.revoked, revokeCall{id: id, replacedBy: replacedBy})
	return nil
}

func (f *fakeRefreshRepo) Insert(_ context.Context, t *domain.RefreshToken) error {
	f.inserted = append(f.inserted, t)
	return nil
}

func (f *fakeRefreshRepo) RevokeAllForUser(context.Context, domain.UserID, time.Time) error {
	return nil
}

type fakeUserRepo struct {
	ports.UserRepository
}

func (fakeUserRepo) GetByID(context.Context, domain.UserID) (*domain.User, error) {
	return domain.NewUser("u-1", "a@b.c", "hash", nil, time.Now()), nil
}

type fakeIssuer struct{}

func (fakeIssuer) Issue(*domain.User) (domain.AccessToken, error) {
	return domain.AccessToken{Value: "new.access", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

type fakeRefreshGen struct{}

func (fakeRefreshGen) Generate() (token, hash string, err error) { return "new-plain", "new-hash", nil }
func (fakeRefreshGen) Hash(t string) string                      { return "sha256:" + t }

type fakeTxManager struct {
	repos ports.RepoSet
	calls int
}

func (f *fakeTxManager) WithinTx(ctx context.Context, fn func(context.Context, ports.RepoSet) error) error {
	f.calls++
	return fn(ctx, f.repos)
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type stubIDGen struct{}

func (stubIDGen) NewID() string { return "rt-new" }

func newUseCase(repo *fakeRefreshRepo, clock fixedClock) *refreshtoken.UseCase {
	tx := &fakeTxManager{repos: ports.RepoSet{RefreshTokens: repo}}
	return refreshtoken.New(fakeUserRepo{}, repo, fakeIssuer{}, fakeRefreshGen{}, tx, clock, stubIDGen{}, 720*time.Hour)
}

func TestRefresh_Success_RotatesInOneTx(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRefreshRepo{stored: &domain.RefreshToken{
		ID:        "rt-old",
		UserID:    "u-1",
		TokenHash: "sha256:presented",
		ExpiresAt: now.Add(time.Hour),
	}}
	tx := &fakeTxManager{repos: ports.RepoSet{RefreshTokens: repo}}
	uc := refreshtoken.New(fakeUserRepo{}, repo, fakeIssuer{}, fakeRefreshGen{}, tx, fixedClock{t: now}, stubIDGen{}, 720*time.Hour)

	resp, err := uc.Execute(context.Background(), ports.RefreshTokenRequest{RefreshToken: "presented"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.AccessToken != "new.access" || resp.RefreshToken != "new-plain" {
		t.Fatalf("response = %+v", resp)
	}
	if tx.calls != 1 {
		t.Fatalf("WithinTx calls = %d, want 1 (revoke+insert atomic)", tx.calls)
	}
	if len(repo.revoked) != 1 || repo.revoked[0].id != "rt-old" {
		t.Fatalf("revoked = %+v, want one revoke of rt-old", repo.revoked)
	}
	if repo.revoked[0].replacedBy == nil || *repo.revoked[0].replacedBy != "rt-new" {
		t.Fatalf("old token must point replaced_by at the new id, got %+v", repo.revoked[0].replacedBy)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].ID != "rt-new" || repo.inserted[0].TokenHash != "new-hash" {
		t.Fatalf("inserted = %+v, want one new token rt-new/new-hash", repo.inserted)
	}
}

func TestRefresh_Rejects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Hour)

	cases := []struct {
		name     string
		repo     *fakeRefreshRepo
		req      string
		wantCode string
	}{
		{
			name:     "expired",
			repo:     &fakeRefreshRepo{stored: &domain.RefreshToken{ID: "rt", UserID: "u-1", ExpiresAt: now.Add(-time.Minute)}},
			req:      "presented",
			wantCode: "identity.token_expired",
		},
		{
			name:     "revoked",
			repo:     &fakeRefreshRepo{stored: &domain.RefreshToken{ID: "rt", UserID: "u-1", ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt}},
			req:      "presented",
			wantCode: "identity.token_revoked",
		},
		{
			name:     "unknown",
			repo:     &fakeRefreshRepo{getErr: errors.NewNotFound("identity.refresh_not_found", "missing", nil)},
			req:      "presented",
			wantCode: "identity.token_invalid",
		},
		{
			name:     "empty",
			repo:     &fakeRefreshRepo{},
			req:      "",
			wantCode: "identity.token_invalid",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := newUseCase(tc.repo, fixedClock{t: now})

			_, err := uc.Execute(context.Background(), ports.RefreshTokenRequest{RefreshToken: tc.req})

			var e *errors.Error
			if !errors.As(err, &e) || e.Code != tc.wantCode {
				t.Fatalf("error = %v, want code %q", err, tc.wantCode)
			}
			if len(tc.repo.revoked) != 0 || len(tc.repo.inserted) != 0 {
				t.Fatalf("no rotation must happen on rejection: revoked=%d inserted=%d", len(tc.repo.revoked), len(tc.repo.inserted))
			}
		})
	}
}
