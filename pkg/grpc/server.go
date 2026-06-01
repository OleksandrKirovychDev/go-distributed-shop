package grpc

import (
	"log/slog"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

type Option func(*config)

type config struct {
	serverOptions     []grpc.ServerOption
	unaryInterceptors []grpc.UnaryServerInterceptor
}

// WithServerOptions appends raw grpc.ServerOptions (TLS creds, keepalive, …).
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(c *config) { c.serverOptions = append(c.serverOptions, opts...) }
}

// WithUnaryInterceptors appends interceptors after the built-in chain, so they
// run closer to the handler than recovery/request-ID/logging.
func WithUnaryInterceptors(in ...grpc.UnaryServerInterceptor) Option {
	return func(c *config) { c.unaryInterceptors = append(c.unaryInterceptors, in...) }
}

func NewServer(log *slog.Logger, opts ...Option) *grpc.Server {
	if log == nil {
		log = slog.Default()
	}

	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	// Outermost first: recovery turns panics into Internal, request-ID seeds
	// the context, logging emits the per-call line with the request-scoped
	// logger handlers reuse.
	chain := []grpc.UnaryServerInterceptor{
		recoveryUnaryInterceptor(log),
		requestIDUnaryInterceptor(),
		loggingUnaryInterceptor(log),
	}
	chain = append(chain, cfg.unaryInterceptors...)

	serverOpts := []grpc.ServerOption{
		// Stats handler (not the deprecated otelgrpc interceptor) is what emits
		// the rpc_* RED metrics and the server span the logger correlates to.
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
		)),
		grpc.ChainUnaryInterceptor(chain...),
	}
	serverOpts = append(serverOpts, cfg.serverOptions...)

	return grpc.NewServer(serverOpts...)
}
