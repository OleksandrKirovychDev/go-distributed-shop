// Package logger provides the project's structured logger. The exported
// Logger is a *slog.Logger backed by a JSON handler that auto-attaches
// trace_id, span_id, request_id, and user_id from the call-site context.
// Services should call New(serviceName) once at startup, store the result,
// and use From(ctx) at every request entry point.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type Option func(*options)

type options struct {
	level  slog.Level
	writer io.Writer
}

func WithLevel(l slog.Level) Option {
	return func(o *options) { o.level = l }
}

func WithWriter(w io.Writer) Option {
	return func(o *options) { o.writer = w }
}

func New(serviceName string, opts ...Option) *slog.Logger {
	cfg := options{level: slog.LevelInfo, writer: os.Stdout}
	for _, opt := range opts {
		opt(&cfg)
	}

	base := slog.NewJSONHandler(cfg.writer, &slog.HandlerOptions{
		Level: cfg.level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "ts"
			}
			return a
		},
	})

	return slog.New(&contextHandler{
		outer: base.WithAttrs([]slog.Attr{slog.String("service", serviceName)}),
	})
}

// contextHandler tracks the caller's WithAttrs/WithGroup chain so context-
// derived attrs (trace_id, span_id, request_id, user_id) can be inserted at
// the top level on every Handle. slog otherwise places Record attrs inside
// any active WithGroup, which would nest the correlation IDs and break
// trace<->log correlation queries on grouped sub-loggers.
type contextHandler struct {
	outer slog.Handler
	chain []chainOp
}

type chainOp struct {
	attrs []slog.Attr
	group string
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.outer.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := h.outer
	if attrs := contextAttrs(ctx); len(attrs) > 0 {
		handler = handler.WithAttrs(attrs)
	}
	for _, op := range h.chain {
		if op.attrs != nil {
			handler = handler.WithAttrs(op.attrs)
		} else {
			handler = handler.WithGroup(op.group)
		}
	}
	return handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &contextHandler{outer: h.outer, chain: appendOp(h.chain, chainOp{attrs: attrs})}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &contextHandler{outer: h.outer, chain: appendOp(h.chain, chainOp{group: name})}
}

func appendOp(chain []chainOp, op chainOp) []chainOp {
	out := make([]chainOp, len(chain)+1)
	copy(out, chain)
	out[len(chain)] = op
	return out
}

func contextAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if id := RequestIDFrom(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if id := UserIDFrom(ctx); id != "" {
		attrs = append(attrs, slog.String("user_id", id))
	}
	return attrs
}

func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func Into(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

type loggerKey struct{}
