package rpc

import (
	"context"
	"nodehub"
	"nodehub/logger"
	"nodehub/proto/clientpb"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// LogUnary 打印unary请求日志
func LogUnary(l logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		defer func() {
			vals := []any{
				"method", info.FullMethod,
				"duration", time.Since(start).String(),
			}

			if md, ok := metadata.FromIncomingContext(ctx); ok {
				if v := md.Get(nodehub.MDUserID); len(v) > 0 {
					vals = append(vals, "userID", v[0])
				}

				if v := md.Get(nodehub.MDTransactionID); len(v) > 0 {
					vals = append(vals, "transID", v[0])
				}
			}

			if v, ok := req.(proto.Message); ok {
				vals = append(vals, "reqType", v.ProtoReflect().Descriptor().FullName())
			}

			if v, ok := resp.(*clientpb.Response); ok {
				vals = append(vals, "respType", v.Type)
			}

			if err != nil {
				vals = append(vals, "error", err)
				l.Error("grpc request", vals...)
			} else {
				l.Info("grpc request", vals...)
			}
		}()

		return handler(ctx, req)
	}
}
