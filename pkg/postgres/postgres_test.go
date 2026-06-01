//go:build integration

package postgres_test

import (
	"context"
	stderrors "errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/online-shop/pkg/postgres"
)

func setupContainer(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	return dsn
}

func TestNewPool_OpensAndPings(t *testing.T) {
	dsn := setupContainer(t)

	pool, err := postgres.NewPool(context.Background(), postgres.Config{
		URL: dsn, MaxConns: 4, MinConns: 1, HealthCheckPeriod: 5 * time.Second, ConnectTimeout: 5 * time.Second,
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	var got int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 1 {
		t.Fatalf("SELECT 1 = %d", got)
	}
}

func TestWithTx_CommitsOnNil(t *testing.T) {
	dsn := setupContainer(t)
	pool, err := postgres.NewPool(context.Background(), postgres.Config{
		URL: dsn, MaxConns: 4, MinConns: 1, HealthCheckPeriod: 5 * time.Second, ConnectTimeout: 5 * time.Second,
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "CREATE TABLE t (id INT PRIMARY KEY)")

	err = postgres.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO t VALUES (1)")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row committed, got %d", n)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	dsn := setupContainer(t)
	pool, err := postgres.NewPool(context.Background(), postgres.Config{
		URL: dsn, MaxConns: 4, MinConns: 1, HealthCheckPeriod: 5 * time.Second, ConnectTimeout: 5 * time.Second,
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "CREATE TABLE t (id INT PRIMARY KEY)")
	sentinel := stderrors.New("rollback me")

	err = postgres.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO t VALUES (1)"); err != nil {
			return err
		}
		return sentinel
	})
	if !stderrors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected rollback (0 rows), got %d", n)
	}
}

func TestWithTx_RollsBackOnPanic(t *testing.T) {
	dsn := setupContainer(t)
	pool, err := postgres.NewPool(context.Background(), postgres.Config{
		URL: dsn, MaxConns: 4, MinConns: 1, HealthCheckPeriod: 5 * time.Second, ConnectTimeout: 5 * time.Second,
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.Exec(ctx, "CREATE TABLE t (id INT PRIMARY KEY)")

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic to propagate")
			}
		}()
		_ = postgres.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, _ = tx.Exec(ctx, "INSERT INTO t VALUES (1)")
			panic("boom")
		})
	}()

	var n int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected rollback after panic (0 rows), got %d", n)
	}
}
