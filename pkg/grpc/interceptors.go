package grpc

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/online-shop/pkg/logger"
)

func recoveryUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Recovery is outermost, so the request-scoped logger is not yet
				// in ctx; the base logger still carries trace_id/span_id.
				log.ErrorContext(ctx, "grpc handler panic",
					"grpc.method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func requestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := requestIDFromIncoming(ctx)
		if id == "" {
			id = uuid.NewString()
		}
		ctx = logger.WithRequestID(ctx, id)
		_ = grpc.SetHeader(ctx, metadata.Pairs(requestIDMetadataKey, id))
		return handler(ctx, req)
	}
}

func loggingUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = logger.Into(ctx, log)
		start := time.Now()

		resp, err := handler(ctx, req)

		code := status.Code(err)
		attrs := []any{
			"grpc.method", info.FullMethod,
			"grpc.code", code.String(),
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000.0,
		}
		l := logger.From(ctx)
		switch {
		case err == nil:
			l.InfoContext(ctx, "grpc call", attrs...)
		case isServerFault(code):
			l.ErrorContext(ctx, "grpc call", append(attrs, "error", err.Error())...)
		default:
			l.WarnContext(ctx, "grpc call", append(attrs, "error", err.Error())...)
		}
		return resp, err
	}
}

func isServerFault(c codes.Code) bool {
	switch c {
	case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
		return true
	default:
		return false
	}
}
