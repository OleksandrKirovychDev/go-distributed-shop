// Package grpc is the inbound gRPC adapter: it maps protobuf messages to and
// from the app's request/response DTOs, calls the use cases, and translates
// domain errors to status codes. It is the only place the identity protobuf
// types appear; the app and domain never see them.
package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	identityv1 "github.com/online-shop/proto/gen/go/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
)

type Server struct {
	identityv1.UnimplementedIdentityServiceServer

	registrar     ports.Registrar
	authenticator ports.Authenticator
	refresher     ports.TokenRefresher
	verifier      ports.TokenVerifier
	getter        ports.UserGetter
}

func NewServer(
	registrar ports.Registrar,
	authenticator ports.Authenticator,
	refresher ports.TokenRefresher,
	verifier ports.TokenVerifier,
	getter ports.UserGetter,
) *Server {
	return &Server{
		registrar:     registrar,
		authenticator: authenticator,
		refresher:     refresher,
		verifier:      verifier,
		getter:        getter,
	}
}

const (
	mdUserID    = "x-user-id"
	mdUserRoles = "x-user-roles"
)

// caller is the authenticated principal as forwarded by the gateway in metadata
// (Step 5 router injects these headers). In Step 4 no caller is present, so
// authorization checks that depend on it deny by default.
type caller struct {
	userID string
	roles  []string
}

func callerFromContext(ctx context.Context) caller {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return caller{}
	}
	c := caller{}
	if v := md.Get(mdUserID); len(v) > 0 {
		c.userID = v[0]
	}
	if v := md.Get(mdUserRoles); len(v) > 0 {
		c.roles = strings.Split(v[0], ",")
	}
	return c
}

func (c caller) isAdmin() bool {
	for _, r := range c.roles {
		if strings.TrimSpace(r) == "admin" {
			return true
		}
	}
	return false
}

func (c caller) canAccessUser(userID string) bool {
	return c.isAdmin() || (c.userID != "" && c.userID == userID)
}

var _ identityv1.IdentityServiceServer = (*Server)(nil)
