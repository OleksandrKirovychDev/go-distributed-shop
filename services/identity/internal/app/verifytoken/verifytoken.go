// Package verifytoken implements access-token verification. An invalid token is
// a valid:false answer, never a transport error.
package verifytoken

import (
	"context"

	"github.com/online-shop/services/identity/internal/app/ports"
)

type UseCase struct {
	parser ports.TokenParser
}

func New(parser ports.TokenParser) *UseCase {
	return &UseCase{parser: parser}
}

func (uc *UseCase) Execute(_ context.Context, req ports.VerifyTokenRequest) (ports.VerifyTokenResponse, error) {
	// VerifyToken answers a yes/no question; an unparseable or expired token is
	// a valid:false answer, not a transport error.
	claims, err := uc.parser.Parse(req.AccessToken)
	if err != nil {
		//nolint:nilerr // an invalid or expired token is a valid:false answer, not an error
		return ports.VerifyTokenResponse{Valid: false}, nil
	}

	roles := make([]string, len(claims.Roles))
	for i, r := range claims.Roles {
		roles[i] = r.String()
	}

	return ports.VerifyTokenResponse{
		Valid:  true,
		UserID: claims.UserID.String(),
		Email:  claims.Email.String(),
		Roles:  roles,
	}, nil
}

var _ ports.TokenVerifier = (*UseCase)(nil)
