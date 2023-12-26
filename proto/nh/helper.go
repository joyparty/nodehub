package nh

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewReply 把proto message打包Reply
func NewReply(msgType int32, msg proto.Message) (*Reply, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &Reply{
		MessageType: msgType,
		Data:        data,
	}, nil
}

// NewMulticast 创建push消息
func NewMulticast(receiver []string, content *Reply) *Multicast {
	return &Multicast{
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

// ResetReply 重置响应对象
func ResetReply(resp *Reply) {
	resp.RequestId = 0
	resp.FromService = 0
	resp.MessageType = 0

	if len(resp.Data) > 0 {
		resp.Data = resp.Data[:0]
	}
}
