package grpc

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const requestIDMetadataKey = "x-request-id"

func requestIDFromIncoming(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get(requestIDMetadataKey); len(vals) > 0 {
		return vals[0]
	}
	return ""
}
