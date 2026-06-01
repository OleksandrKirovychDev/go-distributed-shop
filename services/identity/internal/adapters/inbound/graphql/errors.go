package graphql

import (
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/online-shop/pkg/errors"
)

// errUnauthenticated is returned when a protected resolver runs without a caller
// (the router forwards none for anonymous requests).
var errUnauthenticated = errors.NewUnauthorized("identity.unauthenticated", "authentication required", nil)

// toGraphQLError maps a domain error's Kind to a stable extensions.code, the
// GraphQL analogue of the gRPC adapter's toStatus. Internal/unknown errors
// collapse to a generic message so implementation detail never reaches a client.
// gqlgen attaches the field path/locations when the resolver returns this.
func toGraphQLError(err error) error {
	code := "INTERNAL"
	message := "internal error"

	var domainErr *errors.Error
	if errors.As(err, &domainErr) {
		switch domainErr.Kind {
		case errors.KindInvalid:
			code, message = "INVALID", domainErr.Message
		case errors.KindNotFound:
			code, message = "NOT_FOUND", domainErr.Message
		case errors.KindConflict:
			code, message = "CONFLICT", domainErr.Message
		case errors.KindUnauthorized:
			code, message = "UNAUTHORIZED", domainErr.Message
		case errors.KindForbidden:
			code, message = "FORBIDDEN", domainErr.Message
		}
	}

	return &gqlerror.Error{
		Message:    message,
		Extensions: map[string]any{"code": code},
	}
}
