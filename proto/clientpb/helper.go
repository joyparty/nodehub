package clientpb

import "google.golang.org/protobuf/proto"

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
