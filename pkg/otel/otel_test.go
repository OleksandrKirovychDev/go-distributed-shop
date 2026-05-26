package otel_test

import (
	"context"
	"testing"

	otelglobal "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/online-shop/pkg/otel"
)

func TestInit_SetsGlobalProvidersAndPropagator(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	spans := tracetest.NewInMemoryExporter()
	metrics := sdkmetric.NewManualReader()

	shutdown, err := otel.Init(context.Background(), "test-service",
		otel.WithServiceVersion("0.1.0"),
		otel.WithEnvironment("test"),
		otel.WithTraceExporter(spans),
		otel.WithMetricReader(metrics),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		_ = shutdown(context.Background())
	})

	tp := otelglobal.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Fatalf("global tracer provider not the SDK one: %T", tp)
	}

	prop := otelglobal.GetTextMapPropagator()
	fields := prop.Fields()
	hasTraceparent, hasBaggage := false, false
	for _, f := range fields {
		switch f {
		case "traceparent":
			hasTraceparent = true
		case "baggage":
			hasBaggage = true
		}
	}
	if !hasTraceparent || !hasBaggage {
		t.Fatalf("propagator missing fields: %v", fields)
	}
}

func TestInit_SpansCarryResourceAttributes(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	spans := tracetest.NewInMemoryExporter()
	metrics := sdkmetric.NewManualReader()

	shutdown, err := otel.Init(context.Background(), "checkout",
		otel.WithServiceVersion("1.2.3"),
		otel.WithEnvironment("ci"),
		otel.WithTraceExporter(spans),
		otel.WithMetricReader(metrics),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	tracer := otelglobal.GetTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "op")
	span.End()

	tp := otelglobal.GetTracerProvider().(*sdktrace.TracerProvider)
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	got := spans.GetSpans()
	if len(got) != 1 {
		t.Fatalf("expected 1 span, got %d", len(got))
	}
	res := got[0].Resource
	want := map[string]string{
		"service.name":                "checkout",
		"service.version":             "1.2.3",
		"deployment.environment.name": "ci",
	}
	set := res.Set()
	for k, v := range want {
		got, ok := set.Value(attribute.Key(k))
		if !ok {
			t.Errorf("resource missing %s", k)
			continue
		}
		if got.AsString() != v {
			t.Errorf("resource[%s] = %q, want %q", k, got.AsString(), v)
		}
	}
}

func TestInit_EmptyServiceNameRejected(t *testing.T) {
	if _, err := otel.Init(context.Background(), ""); err == nil {
		t.Fatal("Init should reject empty serviceName")
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	shutdown, err := otel.Init(context.Background(), "svc",
		otel.WithTraceExporter(tracetest.NewInMemoryExporter()),
		otel.WithMetricReader(sdkmetric.NewManualReader()),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown should be a no-op error-free: %v", err)
	}
}

func TestInit_HonoursOtelResourceAttributesEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=4.5.6,k8s.pod.name=test-pod")
	t.Setenv("OTEL_SERVICE_VERSION", "")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")

	spans := tracetest.NewInMemoryExporter()
	shutdown, err := otel.Init(context.Background(), "svc-name",
		otel.WithTraceExporter(spans),
		otel.WithMetricReader(sdkmetric.NewManualReader()),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	tracer := otelglobal.GetTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "op")
	span.End()
	tp := otelglobal.GetTracerProvider().(*sdktrace.TracerProvider)
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	set := spans.GetSpans()[0].Resource.Set()
	want := map[string]string{
		"service.name":    "svc-name", // Init arg wins over env
		"service.version": "4.5.6",    // OTEL_RESOURCE_ATTRIBUTES wins over "dev" fallback
		"k8s.pod.name":    "test-pod", // OTEL_RESOURCE_ATTRIBUTES flows through
	}
	for k, v := range want {
		got, ok := set.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			t.Errorf("resource[%s] = %q, want %q", k, got.AsString(), v)
		}
	}
}
