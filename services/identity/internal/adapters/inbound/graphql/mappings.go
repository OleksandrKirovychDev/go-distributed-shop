package graphql

import (
	"github.com/online-shop/services/identity/internal/adapters/inbound/graphql/model"
	"github.com/online-shop/services/identity/internal/app/ports"
)

func toUserModel(u ports.UserView) *model.User {
	return &model.User{
		ID:        u.ID,
		Email:     u.Email,
		Roles:     u.Roles,
		CreatedAt: u.CreatedAt,
	}
}

func toAuthPayloadModel(r ports.LoginUserResponse) *model.AuthPayload {
	return &model.AuthPayload{
		AccessToken:          r.AccessToken,
		RefreshToken:         r.RefreshToken,
		AccessTokenExpiresAt: r.AccessTokenExpiresAt,
	}
}
