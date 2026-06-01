//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	pkgpg "github.com/online-shop/pkg/postgres"
)

// testPool is a single migrated Postgres shared by every integration test in
// this package. Tests use unique UUID/email values and scope their assertions by
// id, so they neither collide nor see each other's rows.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("identity_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("connection string: %v", err)
	}

	pool, err := pkgpg.NewPool(ctx, pkgpg.Config{
		URL:               dsn,
		MaxConns:          4,
		MinConns:          1,
		HealthCheckPeriod: 5 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}, slog.Default())
	if err != nil {
		log.Fatalf("open pool: %v", err)
	}
	if err := applyMigrations(ctx, pool); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	testPool = pool

	code := m.Run()

	pool.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "migrations", "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}
