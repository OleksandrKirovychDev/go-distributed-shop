package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

// RegisterHealth installs the standard grpc.health.v1.Health service and
// returns the server so the composition root can flip serving status.
func RegisterHealth(s *grpc.Server) *health.Server {
	hs := health.NewServer()
	healthgrpc.RegisterHealthServer(s, hs)
	return hs
}
