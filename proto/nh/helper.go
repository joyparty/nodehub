package nh

import (
	"log/slog"
	"reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// EmptyReply 空响应
	EmptyReply = &emptypb.Empty{}

	replyTypes = map[[2]int32]reflect.Type{}
)

func init() {
	RegisterReplyType(0, int32(ReplyCode_RPC_ERROR), &RPCError{})
}

// RegisterReplyType 注册响应数据编码及类型
func RegisterReplyType(serviceCode int32, code int32, message proto.Message) {
	replyTypes[[2]int32{serviceCode, code}] = reflect.TypeOf(message).Elem()
}

// GetReplyType 根据编码获取对应的响应数据类型
func GetReplyType(serviceCode int32, code int32) (reflect.Type, bool) {
	if v, ok := replyTypes[[2]int32{serviceCode, code}]; ok {
		return v, true
	}
	return nil, false
}

// UnpackReply 解包Reply
func UnpackReply(reply *Reply) (msg proto.Message, ok bool, err error) {
	msgType, ok := GetReplyType(reply.GetServiceCode(), reply.GetCode())
	if !ok {
		return
	}

	msg = reflect.New(msgType).Interface().(proto.Message)
	err = proto.Unmarshal(reply.GetData(), msg)
	return
}

// NewReply 把proto message打包Reply
func NewReply(code int32, msg proto.Message) (*Reply, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return Reply_builder{
		Code: code,
		Data: data,
	}.Build(), nil
}

// NewMulticast 创建push消息
func NewMulticast(receiver []string, content *Reply) *Multicast {
	return Multicast_builder{
		Receiver: receiver,
		Time:     timestamppb.Now(),
		Content:  content,
	}.Build()
}

// LogValue implements slog.LogValuer
func (x *Request) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Int("id", int(x.GetId())),
		slog.Int("service", int(x.GetServiceCode())),
		slog.String("method", x.GetMethod()),
	}

	if nodeID := x.GetNodeId(); nodeID != "" {
		attrs = append(attrs, slog.String("nodeID", nodeID))
	}

	if x.GetNoReply() {
		attrs = append(attrs, slog.Bool("noReply", true))
	}

	if stream := x.GetStream(); stream != "" {
		attrs = append(attrs, slog.String("stream", stream))
	}

	return slog.GroupValue(attrs...)
}

// LogValue implements slog.LogValuer
func (x *Reply) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("reqID", int(x.GetRequestId())),
		slog.Int("service", int(x.GetServiceCode())),
		slog.Int("code", int(x.GetCode())),
	)
}
