package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
)

func (s *Server) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
	// Self-or-admin, enforced here (not in the use case) from the caller claims
	// the gateway forwards. No caller in Step 4 ⇒ denied by default.
	if !callerFromContext(ctx).canAccessUser(req.GetUserId()) {
		return nil, status.Error(codes.PermissionDenied, "not allowed to access this user")
	}

	resp, err := s.getter.Execute(ctx, ports.GetUserRequest{UserID: req.GetUserId()})
	if err != nil {
		return nil, toStatus(err)
	}
	return toGetUserResponsePB(resp), nil
}
