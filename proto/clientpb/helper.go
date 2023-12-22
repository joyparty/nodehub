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

// ResetRequest 重置请求对象
func ResetRequest(req *Request) {
	req.Id = 0
	req.NodeId = ""
	req.ServiceCode = 0
	req.Method = ""

	if len(req.Data) > 0 {
		req.Data = req.Data[:0]
	}
}

// ResetResponse 重置响应对象
func ResetResponse(resp *Response) {
	resp.RequestId = 0
	resp.FromService = 0
	resp.Type = 0

	if len(resp.Data) > 0 {
		resp.Data = resp.Data[:0]
	}
}
