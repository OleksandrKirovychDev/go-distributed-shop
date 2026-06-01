package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/online-shop/pkg/errors"
)

// toStatus maps a domain error's Kind to a gRPC status. Internal/unknown errors
// collapse to a generic message so implementation detail never reaches a caller;
// the full chain is already logged by the logging interceptor.
func toStatus(err error) error {
	var domainErr *errors.Error
	if !errors.As(err, &domainErr) {
		return status.Error(codes.Internal, "internal error")
	}

	switch domainErr.Kind {
	case errors.KindInvalid:
		return status.Error(codes.InvalidArgument, domainErr.Message)
	case errors.KindNotFound:
		return status.Error(codes.NotFound, domainErr.Message)
	case errors.KindConflict:
		return status.Error(codes.AlreadyExists, domainErr.Message)
	case errors.KindUnauthorized:
		return status.Error(codes.Unauthenticated, domainErr.Message)
	case errors.KindForbidden:
		return status.Error(codes.PermissionDenied, domainErr.Message)
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
