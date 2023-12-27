package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/metadata"
)

// GatewayIDInContext 从context中获取gateway id
func GatewayIDInContext(ctx context.Context) string {
	value := metadata.ValueFromIncomingContext(ctx, MDGateway)
	if len(value) == 0 {
		panic(errors.New("gateway id not found in incoming context"))
	}

	return value[0]
}

// UserIDInContext 从context中获取user id
func UserIDInContext(ctx context.Context) string {
	value := metadata.ValueFromIncomingContext(ctx, MDUserID)
	if len(value) == 0 {
		panic(errors.New("user id not found in incoming context"))
	}

	return value[0]
}
