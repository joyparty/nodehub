package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/proto/nh"
	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

func logRequest(ctx context.Context, l logger.Logger, method string) func(err error) {
	start := time.Now()

	return func(err error) {
		vars := []any{
			"method", method,
			"duration", time.Since(start).String(),
		}

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := md.Get(MDSessID); len(v) > 0 {
				vars = append(vars, "sessID", v[0])
			}

			if v := md.Get(MDTransactionID); len(v) > 0 {
				vars = append(vars, "transID", v[0])
			}
		}

		if err != nil {
			vars = append(vars, "error", err)
			l.Error("grpc request", vars...)
		} else {
			l.Info("grpc request", vars...)
		}
	}
}

// LogUnary 打印unary请求日志
func LogUnary(l logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer logRequest(ctx, l, info.FullMethod)(err)

		return handler(ctx, req)
	}
}

// LogStream 打印stream接口请求日志
func LogStream(l logger.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer logRequest(ss.Context(), l, info.FullMethod)(err)

		return handler(srv, ss)
	}
}

// PackReply 自动把返回值转换为nodehub.Reply
func PackReply(replyCodes ...map[string]int32) grpc.UnaryServerInterceptor {
	codes := lo.Assign(replyCodes...)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		resp, err = handler(ctx, req)
		if err != nil {
			return
		}

		if code, ok := codes[info.FullMethod]; ok {
			var reply *nh.Reply
			reply, err = nh.NewReply(code, resp.(proto.Message))
			if err != nil {
				err = fmt.Errorf("pack nodehub.Reply, %w", err)
			}
			resp = reply
		}

		return
	}
}

type replyStream struct {
	grpc.ServerStream

	replyCode int32
}

func (s *replyStream) SendMsg(m any) error {
	reply, err := nh.NewReply(s.replyCode, m.(proto.Message))
	if err != nil {
		return err
	}

	return s.ServerStream.SendMsg(reply)
}

// PackReplyStream 把server side stream返回值转换为nodehub.Reply
func PackReplyStream(replyCodes ...map[string]int32) grpc.StreamServerInterceptor {
	codes := lo.Assign(replyCodes...)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if code, ok := codes[info.FullMethod]; ok {
			return handler(srv, &replyStream{
				ServerStream: ss,
				replyCode:    code,
			})
		}

		return handler(srv, ss)
	}
}
