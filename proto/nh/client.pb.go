// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.21.12
// source: nodehub/client.proto

package nh

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// 客户端上行消息
// 请求会被网关转换为grpc请求转发到内部服务
type Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// id应该按照发送顺序自增长
	// 网关会在每个request对应的reply.request_id内原样返回这个值
	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// 服务ID，对应内部节点的每种grpc服务
	// 内部节点在注册服务发现时，会将服务ID注册到etcd中
	// 网关根据service字段将请求转发到对应的内部服务
	ServiceCode int32 `protobuf:"varint,2,opt,name=service_code,json=serviceCode,proto3" json:"service_code,omitempty"`
	// grpc方法名，大小写敏感，例如: SayHello
	Method string `protobuf:"bytes,3,opt,name=method,proto3" json:"method,omitempty"`
	// grpc方法对应的protobuf message序列化之后的数据
	// 具体对应关系需要自行查看grpc服务的protobuf文件
	Data []byte `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
	// 节点ID
	// 如果有值，网关会把本次请求直接转发到指定的节点
	// 仅仅在有状态服务的allocation配置为client时有效
	NodeId string `protobuf:"bytes,5,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	// 是否需要网关返回response
	NoReply bool `protobuf:"varint,6,opt,name=no_reply,json=noReply,proto3" json:"no_reply,omitempty"`
	// 如果不设置值，网关会马上以并发的方式处理这个请求
	// 如果设置了值，网关会严格按照接收顺序处理同一个流的所有的请求
	// 使用了流以后，性能会受到一定影响，因为网关会等待上一个请求的返回之后才会处理下一个请求
	// 不同的流之间不会互相影响
	// 例如：stream = "stream1"，表示这个请求属于stream1这个流
	Stream string `protobuf:"bytes,7,opt,name=stream,proto3" json:"stream,omitempty"`
}

func (x *Request) Reset() {
	*x = Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_client_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_client_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_nodehub_client_proto_rawDescGZIP(), []int{0}
}

func (x *Request) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Request) GetServiceCode() int32 {
	if x != nil {
		return x.ServiceCode
	}
	return 0
}

func (x *Request) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *Request) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Request) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *Request) GetNoReply() bool {
	if x != nil {
		return x.NoReply
	}
	return false
}

func (x *Request) GetStream() string {
	if x != nil {
		return x.Stream
	}
	return ""
}

// 来自服务器端下行的消息
type Reply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// 触发此次请求的request_id
	// 网关会自动给这个字段赋值
	// 如果是服务器端主动下发，request_id = 0
	RequestId uint32 `protobuf:"varint,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	// 服务ID，对应内部节点的每种grpc服务
	// 标识这个消息来自于哪个内部服务
	// grpc调用返回结果，网关会自动给这个字段赋值
	// 如果是服务器端主动下发，需要自行赋值
	// service_code = 0，表示这个消息来自于网关本身
	ServiceCode int32 `protobuf:"varint,2,opt,name=service_code,json=serviceCode,proto3" json:"service_code,omitempty"`
	// 消息类型代码
	// code = 0，表示这是google.protobuf.Empty类型的空消息
	Code int32 `protobuf:"varint,3,opt,name=code,proto3" json:"code,omitempty"`
	// 下行protobuf message序列化之后的数据
	// 客户端需要根据code字段判断具体反序列化成哪个protobuf message
	Data []byte `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Reply) Reset() {
	*x = Reply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_client_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Reply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Reply) ProtoMessage() {}

func (x *Reply) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_client_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Reply.ProtoReflect.Descriptor instead.
func (*Reply) Descriptor() ([]byte, []int) {
	return file_nodehub_client_proto_rawDescGZIP(), []int{1}
}

func (x *Reply) GetRequestId() uint32 {
	if x != nil {
		return x.RequestId
	}
	return 0
}

func (x *Reply) GetServiceCode() int32 {
	if x != nil {
		return x.ServiceCode
	}
	return 0
}

func (x *Reply) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

func (x *Reply) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_nodehub_client_proto protoreflect.FileDescriptor

var file_nodehub_client_proto_rawDesc = []byte{
	0x0a, 0x14, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x22,
	0xb4, 0x01, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x05, 0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f,
	0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64,
	0x65, 0x49, 0x64, 0x12, 0x19, 0x0a, 0x08, 0x6e, 0x6f, 0x5f, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x6e, 0x6f, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x22, 0x71, 0x0a, 0x05, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12,
	0x1d, 0x0a, 0x0a, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0d, 0x52, 0x09, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x64, 0x12, 0x21,
	0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x64,
	0x65, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x42, 0x26, 0x5a, 0x24, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a, 0x6f, 0x79, 0x70, 0x61, 0x72, 0x74, 0x79,
	0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e,
	0x68, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_nodehub_client_proto_rawDescOnce sync.Once
	file_nodehub_client_proto_rawDescData = file_nodehub_client_proto_rawDesc
)

func file_nodehub_client_proto_rawDescGZIP() []byte {
	file_nodehub_client_proto_rawDescOnce.Do(func() {
		file_nodehub_client_proto_rawDescData = protoimpl.X.CompressGZIP(file_nodehub_client_proto_rawDescData)
	})
	return file_nodehub_client_proto_rawDescData
}

var file_nodehub_client_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_nodehub_client_proto_goTypes = []any{
	(*Request)(nil), // 0: nodehub.Request
	(*Reply)(nil),   // 1: nodehub.Reply
}
var file_nodehub_client_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_nodehub_client_proto_init() }
func file_nodehub_client_proto_init() {
	if File_nodehub_client_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_nodehub_client_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_nodehub_client_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Reply); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_nodehub_client_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_nodehub_client_proto_goTypes,
		DependencyIndexes: file_nodehub_client_proto_depIdxs,
		MessageInfos:      file_nodehub_client_proto_msgTypes,
	}.Build()
	File_nodehub_client_proto = out.File
	file_nodehub_client_proto_rawDesc = nil
	file_nodehub_client_proto_goTypes = nil
	file_nodehub_client_proto_depIdxs = nil
}
