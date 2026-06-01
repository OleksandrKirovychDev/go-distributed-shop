//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/online-shop/services/identity/internal/adapters/outbound/argon2"
	"github.com/online-shop/services/identity/internal/adapters/outbound/events"
	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres"
	"github.com/online-shop/services/identity/internal/adapters/outbound/system"
	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/app/registeruser"
	"github.com/online-shop/services/identity/internal/domain"
)

func fastHasher() *argon2.PasswordHasher {
	return argon2.NewPasswordHasher(argon2.Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
}

// TestRegisterUser_WritesUserAndOutboxAtomically is the outbox keystone: a real
// pool + real TxManager must commit the user row and its outbox event together.
func TestRegisterUser_WritesUserAndOutboxAtomically(t *testing.T) {
	ctx := context.Background()
	uc := registeruser.New(fastHasher(), events.NewEncoder(), postgres.NewTxManager(testPool), system.NewClock(), system.NewIDGenerator())

	email := "outbox-" + uuid.NewString() + "@example.com"
	resp, err := uc.Execute(ctx, ports.RegisterUserRequest{Email: email, Password: "hunter2!"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var dbEmail string
	if err := testPool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", resp.UserID).Scan(&dbEmail); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if dbEmail != email {
		t.Fatalf("user email = %q, want %q", dbEmail, email)
	}

	var topic, aggregateID string
	if err := testPool.QueryRow(ctx,
		"SELECT topic, aggregate_id::text FROM outbox WHERE aggregate_id = $1", resp.UserID,
	).Scan(&topic, &aggregateID); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	if topic != "identity.user.registered.v1" {
		t.Fatalf("outbox topic = %q", topic)
	}
	if aggregateID != resp.UserID {
		t.Fatalf("outbox aggregate_id = %q, want %q", aggregateID, resp.UserID)
	}
}

// malformedEncoder yields an event whose aggregate id is not a UUID, so the
// outbox insert fails inside the tx — exercising rollback.
type malformedEncoder struct{}

func (malformedEncoder) UserRegistered(*domain.User, time.Time) (ports.OutboxEvent, error) {
	return ports.OutboxEvent{AggregateID: "not-a-uuid", Topic: "identity.user.registered.v1", Key: []byte("k"), Payload: []byte("p")}, nil
}

func TestRegisterUser_RollsBackUserWhenOutboxFails(t *testing.T) {
	ctx := context.Background()
	uc := registeruser.New(fastHasher(), malformedEncoder{}, postgres.NewTxManager(testPool), system.NewClock(), system.NewIDGenerator())

	email := "rollback-" + uuid.NewString() + "@example.com"
	if _, err := uc.Execute(ctx, ports.RegisterUserRequest{Email: email, Password: "hunter2!"}); err == nil {
		t.Fatal("expected register to fail when the outbox write fails")
	}

	var count int
	if err := testPool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email = $1", email).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("user row must roll back with the failed outbox write, found %d", count)
	}
}
