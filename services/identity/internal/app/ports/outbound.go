package ports

import (
	"context"
	"time"

	"github.com/online-shop/services/identity/internal/domain"
)

type UserRepository interface {
	Insert(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)
	GetByEmail(ctx context.Context, email domain.Email) (*domain.User, error)
	UpdateEmail(ctx context.Context, id domain.UserID, email domain.Email, now time.Time) error
}

type RefreshTokenRepository interface {
	Insert(ctx context.Context, t *domain.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id domain.RefreshTokenID, now time.Time, replacedBy *domain.RefreshTokenID) error
	RevokeAllForUser(ctx context.Context, id domain.UserID, now time.Time) error
}

type PasswordHasher interface {
	Hash(ctx context.Context, plain domain.PlainPassword) (domain.PasswordHash, error)
	// Verify reports whether plain matches hash. A false with nil error means a
	// clean mismatch; a non-nil error means the hash could not be evaluated.
	Verify(ctx context.Context, plain domain.PlainPassword, hash domain.PasswordHash) (bool, error)
}

// RefreshTokenGenerator mints opaque 256-bit refresh tokens and hashes them for
// storage. Hash is exposed separately so the refresh flow can look up a
// presented token by its hash.
type RefreshTokenGenerator interface {
	Generate() (token string, hash string, err error)
	Hash(token string) string
}

type TokenIssuer interface {
	Issue(user *domain.User) (domain.AccessToken, error)
}

type TokenParser interface {
	Parse(raw string) (domain.Claims, error)
}

// OutboxEvent is the transport-agnostic row the EventPublisher persists. The
// EventEncoder owns the proto marshalling that produces Key/Payload, keeping
// the app layer free of generated event types.
type OutboxEvent struct {
	AggregateID string
	Topic       string
	Key         []byte
	Payload     []byte
	Headers     map[string]string
}

type EventEncoder interface {
	UserRegistered(u *domain.User, now time.Time) (OutboxEvent, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, event OutboxEvent) error
}

// RepoSet is the tx-bound unit of work handed to a WithinTx callback: repos and
// the EventPublisher all share the same pgx.Tx, so a domain write and its
// outbox row commit or roll back together.
type RepoSet struct {
	Users         UserRepository
	RefreshTokens RefreshTokenRepository
	Events        EventPublisher
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, repos RepoSet) error) error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
