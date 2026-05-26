// Package otel initialises the global OpenTelemetry TracerProvider and
// MeterProvider for a service process. The expected production wiring is
// OTLP/gRPC to a local Collector (sidecar or DaemonSet); when no endpoint
// is configured, exporters fall back to stdout so services and the
// smoketest are runnable without infrastructure. Init must be called once
// at startup; the returned shutdown function flushes both providers and
// should be wired into the process's lifecycle.
package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

type ShutdownFunc func(context.Context) error

type Option func(*options)

type options struct {
	serviceVersion string
	environment    string
	traceExporter  sdktrace.SpanExporter
	metricReader   sdkmetric.Reader
	resourceAttrs  []resource.Option
}

func WithServiceVersion(v string) Option {
	return func(o *options) { o.serviceVersion = v }
}

func WithEnvironment(env string) Option {
	return func(o *options) { o.environment = env }
}

func WithTraceExporter(e sdktrace.SpanExporter) Option {
	return func(o *options) { o.traceExporter = e }
}

func WithMetricReader(r sdkmetric.Reader) Option {
	return func(o *options) { o.metricReader = r }
}

func WithResourceAttrs(attrs ...resource.Option) Option {
	return func(o *options) { o.resourceAttrs = append(o.resourceAttrs, attrs...) }
}

func Init(ctx context.Context, serviceName string, opts ...Option) (ShutdownFunc, error) {
	if serviceName == "" {
		return nil, errors.New("otel: serviceName is required")
	}

	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	res, err := buildResource(ctx, serviceName, cfg)
	if err != nil {
		return nil, err
	}

	traceExp, err := pickTraceExporter(ctx, cfg.traceExporter)
	if err != nil {
		return nil, err
	}
	metricReader, err := pickMetricReader(ctx, cfg.metricReader)
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var (
		once    sync.Once
		shutErr error
	)
	shutdown := func(ctx context.Context) error {
		once.Do(func() {
			var errs []error
			tpCtx, cancel := halfDeadline(ctx)
			defer cancel()
			if err := tp.Shutdown(tpCtx); err != nil {
				errs = append(errs, fmt.Errorf("trace provider: %w", err))
			}
			if err := mp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("meter provider: %w", err))
			}
			shutErr = errors.Join(errs...)
		})
		return shutErr
	}
	return shutdown, nil
}

func halfDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx, func() {}
	}
	return context.WithDeadline(ctx, time.Now().Add(time.Until(deadline)/2))
}

func buildResource(ctx context.Context, serviceName string, cfg options) (*resource.Resource, error) {
	opts := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceVersion(envOr("OTEL_SERVICE_VERSION", "dev")),
			semconv.DeploymentEnvironmentName(envOr("DEPLOYMENT_ENVIRONMENT", "local")),
		),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithOS(),
		resource.WithHost(),
		resource.WithFromEnv(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	}
	if cfg.serviceVersion != "" {
		opts = append(opts, resource.WithAttributes(semconv.ServiceVersion(cfg.serviceVersion)))
	}
	if cfg.environment != "" {
		opts = append(opts, resource.WithAttributes(semconv.DeploymentEnvironmentName(cfg.environment)))
	}
	opts = append(opts, cfg.resourceAttrs...)

	res, err := resource.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otel: build resource: %w", err)
	}
	return res, nil
}

func pickTraceExporter(ctx context.Context, override sdktrace.SpanExporter) (sdktrace.SpanExporter, error) {
	if override != nil {
		return override, nil
	}
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		exp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpointURL(endpoint),
			otlptracegrpc.WithInsecure(),
		))
		if err != nil {
			return nil, fmt.Errorf("otel: otlp trace exporter: %w", err)
		}
		return exp, nil
	}
	exp, err := stdouttrace.New()
	if err != nil {
		return nil, fmt.Errorf("otel: stdout trace exporter: %w", err)
	}
	return exp, nil
}

func pickMetricReader(ctx context.Context, override sdkmetric.Reader) (sdkmetric.Reader, error) {
	if override != nil {
		return override, nil
	}
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		exp, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpointURL(endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("otel: otlp metric exporter: %w", err)
		}
		return sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(15*time.Second)), nil
	}
	exp, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("otel: stdout metric exporter: %w", err)
	}
	return sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(15*time.Second)), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
