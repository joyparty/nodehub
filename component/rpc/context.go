package rpc

import (
	"context"
	"errors"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/metadata"
)

// GatewayIDInContext 从context中获取gateway id
func GatewayIDInContext(ctx context.Context) ulid.ULID {
	value := metadata.ValueFromIncomingContext(ctx, MDGateway)
	if len(value) == 0 {
		panic(errors.New("gateway id not found in incoming context"))
	}

	gwID, _ := ulid.Parse(value[0])
	return gwID
}

// SessionIDInContext 从context中获取session id
func SessionIDInContext(ctx context.Context) string {
	value := metadata.ValueFromIncomingContext(ctx, MDSessID)
	if len(value) == 0 {
		panic(errors.New("user id not found in incoming context"))
	}

	return value[0]
}
