package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the minimum subset of pgx's interface that repositories should
// depend on. It lets a repository accept either a *pgxpool.Pool (auto-commit)
// or a pgx.Tx (inside WithTx) without conditional branches at the call site.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func WithTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) (err error) {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		rbCtx := context.WithoutCancel(ctx)
		if r := recover(); r != nil {
			_ = tx.Rollback(rbCtx)
			panic(r)
		}
		switch {
		case err != nil:
			if rbErr := tx.Rollback(rbCtx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("postgres: rollback: %w", rbErr))
			}
		default:
			if cmErr := tx.Commit(ctx); cmErr != nil {
				err = fmt.Errorf("postgres: commit: %w", cmErr)
			}
		}
	}()

	return fn(tx)
}
