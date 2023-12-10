package clientpb

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewResponse 把proto message打包为client.Response
func NewResponse(msgType int32, msg proto.Message) (*Response, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &Response{
		Type: msgType,
		Data: data,
	}, nil
}

// NewNotification 创建push消息
func NewNotification(receiver []string, content *Response) *Notification {
	return &Notification{
		Receiver: receiver,
		Time:     timestamppb.Now(),
		Content:  content,
	}
}
