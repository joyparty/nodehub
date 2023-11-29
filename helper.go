package nodehub

import (
	"nodehub/proto/clientpb"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var requestID = &atomic.Uint32{}

// NewClientRequest 创建一个新的client.Request
func NewClientRequest() *clientpb.Request {
	return &clientpb.Request{
		Id: requestID.Add(1),
	}
}

// SetRequestInfo 设置client.Response的requestId和serviceCode
func SetRequestInfo(resp *clientpb.Response, req *clientpb.Request) {
	resp.RequestId = req.Id
	resp.ServiceCode = req.ServiceCode
}

// PackClientRequest 把上行的proto message打包为client.Request
func PackClientRequest(serviceCode int32, method string, msg proto.Message) (*clientpb.Request, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &clientpb.Request{
		Id:          requestID.Add(1),
		ServiceCode: serviceCode,
		Method:      method,
		Data:        data,
	}, nil
}

// PackClientResponse 把下行的proto message打包为client.Response
func PackClientResponse(msgType int32, msg proto.Message) (*clientpb.Response, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &clientpb.Response{
		Type: msgType,
		Data: data,
	}, nil
}

// NewEmptyMessage 把二进制数据装载为google.proto.Empty
func NewEmptyMessage(data []byte) (*emptypb.Empty, error) {
	msg := &emptypb.Empty{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
