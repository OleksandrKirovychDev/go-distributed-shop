package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgpg "github.com/online-shop/pkg/postgres"

	"github.com/online-shop/services/identity/internal/app/ports"
)

// TxManager runs a unit of work in one transaction. The RepoSet it hands the
// callback is bound to that tx — including the EventPublisher — so a domain
// write and its outbox row commit or roll back together.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context, repos ports.RepoSet) error) error {
	if err := pkgpg.WithTx(ctx, m.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(ctx, ports.RepoSet{
			Users:         NewUserRepository(tx),
			RefreshTokens: NewRefreshTokenRepository(tx),
			Events:        NewEventPublisher(tx),
		})
	}); err != nil {
		return fmt.Errorf("identity tx: %w", err)
	}
	return nil
}

var _ ports.TxManager = (*TxManager)(nil)
