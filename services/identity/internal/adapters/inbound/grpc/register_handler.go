package grpc

import (
	"context"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
)

func (s *Server) RegisterUser(ctx context.Context, req *identityv1.RegisterUserRequest) (*identityv1.RegisterUserResponse, error) {
	resp, err := s.registrar.Execute(ctx, ports.RegisterUserRequest{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return toRegisterResponsePB(resp), nil
}
