package grpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
)

func toRegisterResponsePB(r ports.RegisterUserResponse) *identityv1.RegisterUserResponse {
	return &identityv1.RegisterUserResponse{UserId: r.UserID, Email: r.Email}
}

func toLoginResponsePB(r ports.LoginUserResponse) *identityv1.LoginUserResponse {
	return &identityv1.LoginUserResponse{
		AccessToken:          r.AccessToken,
		RefreshToken:         r.RefreshToken,
		AccessTokenExpiresAt: timestamppb.New(r.AccessTokenExpiresAt),
	}
}

func toRefreshResponsePB(r ports.RefreshTokenResponse) *identityv1.RefreshTokenResponse {
	return &identityv1.RefreshTokenResponse{
		AccessToken:          r.AccessToken,
		RefreshToken:         r.RefreshToken,
		AccessTokenExpiresAt: timestamppb.New(r.AccessTokenExpiresAt),
	}
}

func toVerifyResponsePB(r ports.VerifyTokenResponse) *identityv1.VerifyTokenResponse {
	return &identityv1.VerifyTokenResponse{
		Valid:  r.Valid,
		UserId: r.UserID,
		Roles:  r.Roles,
		Email:  r.Email,
	}
}

func toGetUserResponsePB(r ports.GetUserResponse) *identityv1.GetUserResponse {
	return &identityv1.GetUserResponse{User: &identityv1.User{
		Id:        r.User.ID,
		Email:     r.User.Email,
		Roles:     r.User.Roles,
		CreatedAt: timestamppb.New(r.User.CreatedAt),
	}}
}
