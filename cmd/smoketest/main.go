// Command smoketest emits one structured log line and one OTel span so the
// pkg foundations can be verified end-to-end without standing up a service.
// It is retired at the end of Step 2.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	otelglobal "go.opentelemetry.io/otel"

	"github.com/online-shop/pkg/logger"
	"github.com/online-shop/pkg/otel"
)

func main() {
	if err := run(); err != nil {
		slog.Error("smoketest failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdown, err := otel.Init(ctx, "smoketest",
		otel.WithServiceVersion("0.1.0"),
		otel.WithEnvironment("local"),
	)
	if err != nil {
		return fmt.Errorf("otel init: %w", err)
	}
	defer func() {
		if sErr := shutdown(context.Background()); sErr != nil {
			slog.Error("otel shutdown", "err", sErr)
		}
	}()

	l := logger.New("smoketest")
	tracer := otelglobal.Tracer("smoketest")
	ctx, span := tracer.Start(ctx, "smoketest.boot")

	l.InfoContext(ctx, "hello", "note", "pkg foundations are wired")

	span.End()
	return nil
}
