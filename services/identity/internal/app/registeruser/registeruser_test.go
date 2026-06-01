package registeruser_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/online-shop/pkg/errors"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/app/registeruser"
	"github.com/online-shop/services/identity/internal/domain"
)

type fakeHasher struct {
	ports.PasswordHasher
	hash domain.PasswordHash
	err  error
}

func (f fakeHasher) Hash(context.Context, domain.PlainPassword) (domain.PasswordHash, error) {
	return f.hash, f.err
}

type fakeEncoder struct {
	gotUser *domain.User
	event   ports.OutboxEvent
	err     error
}

func (f *fakeEncoder) UserRegistered(u *domain.User, _ time.Time) (ports.OutboxEvent, error) {
	f.gotUser = u
	return f.event, f.err
}

type fakeUserRepo struct {
	ports.UserRepository
	inserted  *domain.User
	insertErr error
}

func (f *fakeUserRepo) Insert(_ context.Context, u *domain.User) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = u
	return nil
}

type fakeEventPublisher struct {
	published []ports.OutboxEvent
}

func (f *fakeEventPublisher) Publish(_ context.Context, e ports.OutboxEvent) error {
	f.published = append(f.published, e)
	return nil
}

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

type seqIDGen struct{ n int }

func (g *seqIDGen) NewID() string {
	g.n++
	return fmt.Sprintf("id-%d", g.n)
}

func TestRegister_Success(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{}
	pub := &fakeEventPublisher{}
	encoder := &fakeEncoder{event: ports.OutboxEvent{Topic: "identity.user.registered.v1", AggregateID: "agg"}}
	tx := &fakeTxManager{repos: ports.RepoSet{Users: userRepo, Events: pub}}
	clock := fixedClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	uc := registeruser.New(fakeHasher{hash: "argon2-hash"}, encoder, tx, clock, &seqIDGen{})

	resp, err := uc.Execute(context.Background(), ports.RegisterUserRequest{Email: "A@B.Com", Password: "hunter2!"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if resp.Email != "a@b.com" {
		t.Fatalf("response email = %q, want normalised a@b.com", resp.Email)
	}
	if resp.UserID != "id-1" {
		t.Fatalf("response user id = %q, want id-1", resp.UserID)
	}
	if tx.calls != 1 {
		t.Fatalf("WithinTx called %d times, want 1", tx.calls)
	}
	if userRepo.inserted == nil {
		t.Fatal("user was not inserted inside the tx")
	}
	if userRepo.inserted.Email != "a@b.com" || userRepo.inserted.PasswordHash != "argon2-hash" {
		t.Fatalf("inserted user = %+v", userRepo.inserted)
	}
	if len(pub.published) != 1 {
		t.Fatalf("published %d events, want exactly 1", len(pub.published))
	}
	if pub.published[0].Topic != "identity.user.registered.v1" {
		t.Fatalf("event topic = %q", pub.published[0].Topic)
	}
	if encoder.gotUser == nil || encoder.gotUser.ID != "id-1" {
		t.Fatalf("encoder got user = %+v", encoder.gotUser)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{insertErr: domain.ErrEmailTaken}
	pub := &fakeEventPublisher{}
	encoder := &fakeEncoder{event: ports.OutboxEvent{Topic: "identity.user.registered.v1"}}
	tx := &fakeTxManager{repos: ports.RepoSet{Users: userRepo, Events: pub}}

	uc := registeruser.New(fakeHasher{hash: "h"}, encoder, tx, fixedClock{}, &seqIDGen{})

	_, err := uc.Execute(context.Background(), ports.RegisterUserRequest{Email: "a@b.c", Password: "hunter2!"})
	if !errors.IsKind(err, errors.KindConflict) {
		t.Fatalf("error = %v, want Conflict", err)
	}
	if len(pub.published) != 0 {
		t.Fatalf("no event must be published when the user insert fails, got %d", len(pub.published))
	}
}

func TestRegister_RejectsBadInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  ports.RegisterUserRequest
	}{
		{"invalid email", ports.RegisterUserRequest{Email: "nope", Password: "hunter2!"}},
		{"weak password", ports.RegisterUserRequest{Email: "a@b.c", Password: "short"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tx := &fakeTxManager{}
			uc := registeruser.New(fakeHasher{hash: "h"}, &fakeEncoder{}, tx, fixedClock{}, &seqIDGen{})

			_, err := uc.Execute(context.Background(), tc.req)
			if !errors.IsKind(err, errors.KindInvalid) {
				t.Fatalf("error = %v, want Invalid", err)
			}
			if tx.calls != 0 {
				t.Fatalf("WithinTx must not run on bad input, calls = %d", tx.calls)
			}
		})
	}
}
