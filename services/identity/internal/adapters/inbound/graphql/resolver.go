// Package graphql is the inbound GraphQL adapter: it serves the federated
// identity subgraph by mapping GraphQL inputs to the app's request DTOs, calling
// the same use cases the gRPC adapter calls, and translating domain errors to
// GraphQL extensions.code. It is the only place the gqlgen-generated types
// appear; the app and domain never see them.
package graphql

import (
	"go.opentelemetry.io/otel"

	"github.com/online-shop/services/identity/internal/app/ports"
)

// tracer names the use-case spans this adapter opens around each Execute. Span
// creation lives in the adapter so the app layer stays transport-agnostic.
var tracer = otel.Tracer("identity/graphql")

// Resolver carries the inbound ports the field resolvers drive. It mirrors the
// gRPC server's shape, minus the refresh/verify use cases the schema omits.
type Resolver struct {
	registrar     ports.Registrar
	authenticator ports.Authenticator
	userGetter    ports.UserGetter
}

func NewResolver(registrar ports.Registrar, authenticator ports.Authenticator, userGetter ports.UserGetter) *Resolver {
	return &Resolver{registrar: registrar, authenticator: authenticator, userGetter: userGetter}
}
