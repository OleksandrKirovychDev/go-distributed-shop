package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/online-shop/pkg/logger"
)

func TestRequestIDInterceptor_UsesIncomingHeader(t *testing.T) {
	t.Parallel()

	interceptor := requestIDUnaryInterceptor()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(requestIDMetadataKey, "req-abc"))

	var seen string
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(ctx context.Context, _ any) (any, error) {
			seen = logger.RequestIDFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if seen != "req-abc" {
		t.Fatalf("request id = %q, want req-abc", seen)
	}
}

func TestRequestIDInterceptor_GeneratesWhenAbsent(t *testing.T) {
	t.Parallel()

	interceptor := requestIDUnaryInterceptor()

	var seen string
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(ctx context.Context, _ any) (any, error) {
			seen = logger.RequestIDFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if seen == "" {
		t.Fatal("expected a generated request id, got empty")
	}
}

func TestRecoveryInterceptor_TranslatesPanic(t *testing.T) {
	t.Parallel()

	interceptor := recoveryUnaryInterceptor(logger.New("test"))

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) { panic("boom") })
	if resp != nil {
		t.Fatalf("resp = %v, want nil", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}

func TestLoggingInterceptor_InjectsRequestScopedLogger(t *testing.T) {
	t.Parallel()

	base := logger.New("test")
	interceptor := loggingUnaryInterceptor(base)

	var injected bool
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(ctx context.Context, _ any) (any, error) {
			injected = logger.From(ctx) == base
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if !injected {
		t.Fatal("handler did not see the request-scoped logger via logger.From(ctx)")
	}
}
