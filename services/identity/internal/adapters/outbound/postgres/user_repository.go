package postgres

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/online-shop/pkg/errors"
	pkgpg "github.com/online-shop/pkg/postgres"

	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres/gen"
	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

type UserRepository struct {
	q *gen.Queries
}

func NewUserRepository(db pkgpg.Querier) *UserRepository {
	return &UserRepository{q: gen.New(db)}
}

func (r *UserRepository) Insert(ctx context.Context, u *domain.User) error {
	id, err := uuid.Parse(u.ID.String())
	if err != nil {
		return errors.NewInternal("identity.bad_user_id", "user id is not a uuid", err)
	}

	err = r.q.InsertUser(ctx, gen.InsertUserParams{
		ID:           id,
		Email:        u.Email.String(),
		PasswordHash: u.PasswordHash.String(),
		Roles:        rolesToStrings(u.Roles),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	uid, err := uuid.Parse(id.String())
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	row, err := r.q.GetUserByID(ctx, uid)
	if err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return toDomainUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email.String())
	if err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return toDomainUser(row)
}

func (r *UserRepository) UpdateEmail(ctx context.Context, id domain.UserID, email domain.Email, now time.Time) error {
	uid, err := uuid.Parse(id.String())
	if err != nil {
		return domain.ErrUserNotFound
	}
	if err := r.q.UpdateUserEmail(ctx, gen.UpdateUserEmailParams{
		ID:        uid,
		Email:     email.String(),
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("update user email: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return stderrors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ ports.UserRepository = (*UserRepository)(nil)
