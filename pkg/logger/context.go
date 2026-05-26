package logger

import "context"

type ctxKey uint8

const (
	requestIDKey ctxKey = iota
	userIDKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

func WithUserID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDKey, id)
}

func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func UserIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}
