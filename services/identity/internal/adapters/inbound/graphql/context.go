package graphql

import "context"

type contextKey int

const callerContextKey contextKey = iota

// caller is the authenticated principal the router forwards as headers after it
// validates the JWT at the edge. It is absent on public operations
// (registerUser/loginUser); resolvers that require it return unauthenticated.
type caller struct {
	userID string
	roles  []string
}

func withCaller(ctx context.Context, c caller) context.Context {
	return context.WithValue(ctx, callerContextKey, c)
}

func callerFromContext(ctx context.Context) (caller, bool) {
	c, ok := ctx.Value(callerContextKey).(caller)
	return c, ok
}
