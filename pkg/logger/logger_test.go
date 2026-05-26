package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/online-shop/pkg/logger"
)

func decodeLast(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	dec := json.NewDecoder(buf)
	var out map[string]any
	for dec.More() {
		out = nil
		if err := dec.Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	if out == nil {
		t.Fatal("no log line emitted")
	}
	return out
}

func TestNew_EmitsServiceAndStandardFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("checkout", logger.WithWriter(&buf), logger.WithLevel(slog.LevelDebug))
	l.Info("hello", "k", "v")

	got := decodeLast(t, &buf)
	if got["service"] != "checkout" {
		t.Fatalf("service: got %v want checkout", got["service"])
	}
	if got["msg"] != "hello" {
		t.Fatalf("msg: got %v want hello", got["msg"])
	}
	if got["level"] != "INFO" {
		t.Fatalf("level: got %v want INFO", got["level"])
	}
	if got["k"] != "v" {
		t.Fatalf("k: got %v want v", got["k"])
	}
	if _, ok := got["ts"]; !ok {
		t.Fatal("expected ts field (time key renamed)")
	}
}

func TestContextFields_RequestAndUserID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))

	ctx := logger.WithRequestID(context.Background(), "req-123")
	ctx = logger.WithUserID(ctx, "user-456")

	l.InfoContext(ctx, "in-ctx")

	got := decodeLast(t, &buf)
	if got["request_id"] != "req-123" {
		t.Fatalf("request_id: got %v", got["request_id"])
	}
	if got["user_id"] != "user-456" {
		t.Fatalf("user_id: got %v", got["user_id"])
	}
}

func TestContextFields_OmittedWhenAbsent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))
	l.InfoContext(context.Background(), "no-ctx")

	got := decodeLast(t, &buf)
	if _, present := got["request_id"]; present {
		t.Fatal("request_id should be omitted when absent")
	}
	if _, present := got["user_id"]; present {
		t.Fatal("user_id should be omitted when absent")
	}
	if _, present := got["trace_id"]; present {
		t.Fatal("trace_id should be omitted when no span is active")
	}
}

func TestContextFields_TraceFromSpan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))

	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "op")
	defer span.End()

	traceID := trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	ctx = trace.ContextWithSpanContext(ctx, sc)

	l.InfoContext(ctx, "with-trace")

	got := decodeLast(t, &buf)
	if got["trace_id"] != traceID.String() {
		t.Fatalf("trace_id: got %v want %v", got["trace_id"], traceID.String())
	}
	if got["span_id"] != spanID.String() {
		t.Fatalf("span_id: got %v want %v", got["span_id"], spanID.String())
	}
}

func TestFrom_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	if logger.From(context.Background()) != slog.Default() {
		t.Fatal("From(empty ctx) should return slog.Default()")
	}

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))
	ctx := logger.Into(context.Background(), l)
	if logger.From(ctx) != l {
		t.Fatal("From should return the injected logger")
	}
}

func TestEmptyIDsDoNotPolluteContext(t *testing.T) {
	t.Parallel()

	ctx := logger.WithRequestID(context.Background(), "")
	if logger.RequestIDFrom(ctx) != "" {
		t.Fatal("WithRequestID(\"\") must not store an empty value")
	}
	ctx = logger.WithUserID(context.Background(), "")
	if logger.UserIDFrom(ctx) != "" {
		t.Fatal("WithUserID(\"\") must not store an empty value")
	}
}

func TestWithGroup_KeepsContextFieldsAtTopLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))

	traceID := trace.TraceID{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	spanID := trace.SpanID{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = logger.WithRequestID(ctx, "req-1")

	l.WithGroup("db").InfoContext(ctx, "query", "table", "users")

	got := decodeLast(t, &buf)
	if got["service"] != "svc" {
		t.Fatalf("service should be at top level, got: %+v", got)
	}
	if got["trace_id"] != traceID.String() {
		t.Fatalf("trace_id should be at top level, got: %+v", got)
	}
	if got["span_id"] != spanID.String() {
		t.Fatalf("span_id should be at top level, got: %+v", got)
	}
	if got["request_id"] != "req-1" {
		t.Fatalf("request_id should be at top level, got: %+v", got)
	}
	db, ok := got["db"].(map[string]any)
	if !ok {
		t.Fatalf("expected `db` group, got %+v", got)
	}
	if db["table"] != "users" {
		t.Fatalf("table should be inside `db` group, got %+v", db)
	}
	if _, leaked := db["trace_id"]; leaked {
		t.Fatalf("trace_id leaked into group, got %+v", db)
	}
}

func TestWithAttrs_AppliesAtCallerDepth(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := logger.New("svc", logger.WithWriter(&buf))

	l.With("rev", "abc").WithGroup("db").With("table", "users").Info("hit")

	got := decodeLast(t, &buf)
	if got["rev"] != "abc" {
		t.Fatalf("rev should be at top level, got %+v", got)
	}
	db, ok := got["db"].(map[string]any)
	if !ok {
		t.Fatalf("expected `db` group, got %+v", got)
	}
	if db["table"] != "users" {
		t.Fatalf("table should be inside `db` group, got %+v", db)
	}
}
