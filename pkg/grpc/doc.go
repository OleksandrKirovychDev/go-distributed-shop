// Package grpc builds the project's gRPC servers. NewServer wires the
// otelgrpc stats handler (the source of the rpc_* RED metrics and server
// spans) and a fixed unary interceptor chain — recovery, request-ID,
// logging — so every service gets identical cross-cutting behaviour without
// per-service boilerplate. It is deliberately server-side only: client
// builders and client/auth interceptors arrive when a service first needs
// to call out or to trust callers.
package grpc
