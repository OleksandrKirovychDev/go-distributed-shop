package grpc

import (
	"context"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
)

func (s *Server) VerifyToken(ctx context.Context, req *identityv1.VerifyTokenRequest) (*identityv1.VerifyTokenResponse, error) {
	resp, err := s.verifier.Execute(ctx, ports.VerifyTokenRequest{
		AccessToken: req.GetAccessToken(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return toVerifyResponsePB(resp), nil
}
