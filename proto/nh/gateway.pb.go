// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.21.12
// source: nodehub/gateway.proto

package nh

import (
	status "google.golang.org/genproto/googleapis/rpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ReplyCode int32

const (
	ReplyCode_UNSPECIFIED ReplyCode = 0
	ReplyCode_RPC_ERROR   ReplyCode = 1
)

// Enum value maps for ReplyCode.
var (
	ReplyCode_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "RPC_ERROR",
	}
	ReplyCode_value = map[string]int32{
		"UNSPECIFIED": 0,
		"RPC_ERROR":   1,
	}
)

func (x ReplyCode) Enum() *ReplyCode {
	p := new(ReplyCode)
	*p = x
	return p
}

func (x ReplyCode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ReplyCode) Descriptor() protoreflect.EnumDescriptor {
	return file_nodehub_gateway_proto_enumTypes[0].Descriptor()
}

func (ReplyCode) Type() protoreflect.EnumType {
	return &file_nodehub_gateway_proto_enumTypes[0]
}

func (x ReplyCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ReplyCode.Descriptor instead.
func (ReplyCode) EnumDescriptor() ([]byte, []int) {
	return file_nodehub_gateway_proto_rawDescGZIP(), []int{0}
}

// 网关透传grpc请求后，返回的grpc错误
type RPCError struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// request消息请求的服务
	RequestService int32 `protobuf:"varint,1,opt,name=request_service,json=requestService,proto3" json:"request_service,omitempty"`
	// request消息请求的方法
	RequestMethod string `protobuf:"bytes,2,opt,name=request_method,json=requestMethod,proto3" json:"request_method,omitempty"`
	// 详细错误信息
	Status *status.Status `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *RPCError) Reset() {
	*x = RPCError{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RPCError) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RPCError) ProtoMessage() {}

func (x *RPCError) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RPCError.ProtoReflect.Descriptor instead.
func (*RPCError) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_proto_rawDescGZIP(), []int{0}
}

func (x *RPCError) GetRequestService() int32 {
	if x != nil {
		return x.RequestService
	}
	return 0
}

func (x *RPCError) GetRequestMethod() string {
	if x != nil {
		return x.RequestMethod
	}
	return ""
}

func (x *RPCError) GetStatus() *status.Status {
	if x != nil {
		return x.Status
	}
	return nil
}

// 用于内部节点主动向客户端发送消息
// 内部节点把消息打包为Multicast，然后push到消息队列
// 网关节点从消息队列中获取Multicast，然后push到客户端
type Multicast struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// 消息接收者，网关节点根据user_id找到对应的客户端连接
	Receiver []string `protobuf:"bytes,1,rep,name=receiver,proto3" json:"receiver,omitempty"`
	// 消息发布时间
	Time *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=time,proto3" json:"time,omitempty"`
	// 消息内容
	Content *Reply `protobuf:"bytes,3,opt,name=content,proto3" json:"content,omitempty"`
	// 可选，同一个stream的消息会严格按照顺序处理
	Stream string `protobuf:"bytes,4,opt,name=stream,proto3" json:"stream,omitempty"`
	// 可选，是否广播给所有人
	// 如果 receiver 有值，则忽略此字段
	ToEveryone bool `protobuf:"varint,5,opt,name=to_everyone,json=toEveryone,proto3" json:"to_everyone,omitempty"`
}

func (x *Multicast) Reset() {
	*x = Multicast{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Multicast) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Multicast) ProtoMessage() {}

func (x *Multicast) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Multicast.ProtoReflect.Descriptor instead.
func (*Multicast) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_proto_rawDescGZIP(), []int{1}
}

func (x *Multicast) GetReceiver() []string {
	if x != nil {
		return x.Receiver
	}
	return nil
}

func (x *Multicast) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

func (x *Multicast) GetContent() *Reply {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *Multicast) GetStream() string {
	if x != nil {
		return x.Stream
	}
	return ""
}

func (x *Multicast) GetToEveryone() bool {
	if x != nil {
		return x.ToEveryone
	}
	return false
}

var File_nodehub_gateway_proto protoreflect.FileDescriptor

var file_nodehub_gateway_proto_rawDesc = []byte{
	0x0a, 0x15, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x67, 0x61, 0x74, 0x65, 0x77, 0x61,
	0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62,
	0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x17, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x72, 0x70, 0x63, 0x2f, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x14, 0x6e, 0x6f, 0x64, 0x65,
	0x68, 0x75, 0x62, 0x2f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x86, 0x01, 0x0a, 0x08, 0x52, 0x50, 0x43, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x27, 0x0a,
	0x0f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0e, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x53,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x25, 0x0a, 0x0e, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x5f, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x2a, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0xba, 0x01, 0x0a, 0x09, 0x4d, 0x75,
	0x6c, 0x74, 0x69, 0x63, 0x61, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x63, 0x65, 0x69,
	0x76, 0x65, 0x72, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x63, 0x65, 0x69,
	0x76, 0x65, 0x72, 0x12, 0x2e, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x04, 0x74,
	0x69, 0x6d, 0x65, 0x12, 0x28, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2e, 0x52,
	0x65, 0x70, 0x6c, 0x79, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x12, 0x16, 0x0a,
	0x06, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x1f, 0x0a, 0x0b, 0x74, 0x6f, 0x5f, 0x65, 0x76, 0x65, 0x72,
	0x79, 0x6f, 0x6e, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x74, 0x6f, 0x45, 0x76,
	0x65, 0x72, 0x79, 0x6f, 0x6e, 0x65, 0x2a, 0x2b, 0x0a, 0x09, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x43,
	0x6f, 0x64, 0x65, 0x12, 0x0f, 0x0a, 0x0b, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x0d, 0x0a, 0x09, 0x52, 0x50, 0x43, 0x5f, 0x45, 0x52, 0x52, 0x4f,
	0x52, 0x10, 0x01, 0x42, 0x26, 0x5a, 0x24, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x6a, 0x6f, 0x79, 0x70, 0x61, 0x72, 0x74, 0x79, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68,
	0x75, 0x62, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x68, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_nodehub_gateway_proto_rawDescOnce sync.Once
	file_nodehub_gateway_proto_rawDescData = file_nodehub_gateway_proto_rawDesc
)

func file_nodehub_gateway_proto_rawDescGZIP() []byte {
	file_nodehub_gateway_proto_rawDescOnce.Do(func() {
		file_nodehub_gateway_proto_rawDescData = protoimpl.X.CompressGZIP(file_nodehub_gateway_proto_rawDescData)
	})
	return file_nodehub_gateway_proto_rawDescData
}

var file_nodehub_gateway_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_nodehub_gateway_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_nodehub_gateway_proto_goTypes = []any{
	(ReplyCode)(0),                // 0: nodehub.ReplyCode
	(*RPCError)(nil),              // 1: nodehub.RPCError
	(*Multicast)(nil),             // 2: nodehub.Multicast
	(*status.Status)(nil),         // 3: google.rpc.Status
	(*timestamppb.Timestamp)(nil), // 4: google.protobuf.Timestamp
	(*Reply)(nil),                 // 5: nodehub.Reply
}
var file_nodehub_gateway_proto_depIdxs = []int32{
	3, // 0: nodehub.RPCError.status:type_name -> google.rpc.Status
	4, // 1: nodehub.Multicast.time:type_name -> google.protobuf.Timestamp
	5, // 2: nodehub.Multicast.content:type_name -> nodehub.Reply
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_nodehub_gateway_proto_init() }
func file_nodehub_gateway_proto_init() {
	if File_nodehub_gateway_proto != nil {
		return
	}
	file_nodehub_client_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_nodehub_gateway_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*RPCError); i {
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
		file_nodehub_gateway_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Multicast); i {
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
			RawDescriptor: file_nodehub_gateway_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_nodehub_gateway_proto_goTypes,
		DependencyIndexes: file_nodehub_gateway_proto_depIdxs,
		EnumInfos:         file_nodehub_gateway_proto_enumTypes,
		MessageInfos:      file_nodehub_gateway_proto_msgTypes,
	}.Build()
	File_nodehub_gateway_proto = out.File
	file_nodehub_gateway_proto_rawDesc = nil
	file_nodehub_gateway_proto_goTypes = nil
	file_nodehub_gateway_proto_depIdxs = nil
}
