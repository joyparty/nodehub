// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.21.12
// source: room/service.proto

package roompb

import (
	clusterpb "github.com/joyparty/nodehub/example/chat/proto/clusterpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
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
	ReplyCode_NEWS        ReplyCode = 1
)

// Enum value maps for ReplyCode.
var (
	ReplyCode_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "NEWS",
	}
	ReplyCode_value = map[string]int32{
		"UNSPECIFIED": 0,
		"NEWS":        1,
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
	return file_room_service_proto_enumTypes[0].Descriptor()
}

func (ReplyCode) Type() protoreflect.EnumType {
	return &file_room_service_proto_enumTypes[0]
}

func (x ReplyCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ReplyCode.Descriptor instead.
func (ReplyCode) EnumDescriptor() ([]byte, []int) {
	return file_room_service_proto_rawDescGZIP(), []int{0}
}

type JoinRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *JoinRequest) Reset() {
	*x = JoinRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_room_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinRequest) ProtoMessage() {}

func (x *JoinRequest) ProtoReflect() protoreflect.Message {
	mi := &file_room_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinRequest.ProtoReflect.Descriptor instead.
func (*JoinRequest) Descriptor() ([]byte, []int) {
	return file_room_service_proto_rawDescGZIP(), []int{0}
}

func (x *JoinRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type SayRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	To      string `protobuf:"bytes,1,opt,name=to,proto3" json:"to,omitempty"`
	Content string `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
}

func (x *SayRequest) Reset() {
	*x = SayRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_room_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SayRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SayRequest) ProtoMessage() {}

func (x *SayRequest) ProtoReflect() protoreflect.Message {
	mi := &file_room_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SayRequest.ProtoReflect.Descriptor instead.
func (*SayRequest) Descriptor() ([]byte, []int) {
	return file_room_service_proto_rawDescGZIP(), []int{1}
}

func (x *SayRequest) GetTo() string {
	if x != nil {
		return x.To
	}
	return ""
}

func (x *SayRequest) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

type News struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FromId   string `protobuf:"bytes,1,opt,name=from_id,json=fromId,proto3" json:"from_id,omitempty"`
	FromName string `protobuf:"bytes,2,opt,name=from_name,json=fromName,proto3" json:"from_name,omitempty"`
	Content  string `protobuf:"bytes,3,opt,name=content,proto3" json:"content,omitempty"`
}

func (x *News) Reset() {
	*x = News{}
	if protoimpl.UnsafeEnabled {
		mi := &file_room_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *News) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*News) ProtoMessage() {}

func (x *News) ProtoReflect() protoreflect.Message {
	mi := &file_room_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use News.ProtoReflect.Descriptor instead.
func (*News) Descriptor() ([]byte, []int) {
	return file_room_service_proto_rawDescGZIP(), []int{2}
}

func (x *News) GetFromId() string {
	if x != nil {
		return x.FromId
	}
	return ""
}

func (x *News) GetFromName() string {
	if x != nil {
		return x.FromName
	}
	return ""
}

func (x *News) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

var file_room_service_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*clusterpb.Services)(nil),
		Field:         60000,
		Name:          "room.reply_service",
		Tag:           "varint,60000,opt,name=reply_service,enum=cluster.Services",
		Filename:      "room/service.proto",
	},
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*ReplyCode)(nil),
		Field:         60001,
		Name:          "room.reply_code",
		Tag:           "varint,60001,opt,name=reply_code,enum=room.ReplyCode",
		Filename:      "room/service.proto",
	},
}

// Extension fields to descriptorpb.MessageOptions.
var (
	// optional cluster.Services reply_service = 60000;
	E_ReplyService = &file_room_service_proto_extTypes[0]
	// optional room.ReplyCode reply_code = 60001;
	E_ReplyCode = &file_room_service_proto_extTypes[1]
)

var File_room_service_proto protoreflect.FileDescriptor

var file_room_service_proto_rawDesc = []byte{
	0x0a, 0x12, 0x72, 0x6f, 0x6f, 0x6d, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x72, 0x6f, 0x6f, 0x6d, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d,
	0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x16, 0x63, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x21, 0x0a, 0x0b, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x22, 0x36, 0x0a, 0x0a, 0x53, 0x61, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x74, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x74, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x22, 0x60, 0x0a, 0x04,
	0x4e, 0x65, 0x77, 0x73, 0x12, 0x17, 0x0a, 0x07, 0x66, 0x72, 0x6f, 0x6d, 0x5f, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x66, 0x72, 0x6f, 0x6d, 0x49, 0x64, 0x12, 0x1b, 0x0a,
	0x09, 0x66, 0x72, 0x6f, 0x6d, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x66, 0x72, 0x6f, 0x6d, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x74, 0x3a, 0x08, 0x80, 0xa6, 0x1d, 0x01, 0x88, 0xa6, 0x1d, 0x01, 0x2a, 0x26,
	0x0a, 0x09, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x0f, 0x0a, 0x0b, 0x55,
	0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04,
	0x4e, 0x45, 0x57, 0x53, 0x10, 0x01, 0x32, 0xa9, 0x01, 0x0a, 0x04, 0x52, 0x6f, 0x6f, 0x6d, 0x12,
	0x31, 0x0a, 0x04, 0x4a, 0x6f, 0x69, 0x6e, 0x12, 0x11, 0x2e, 0x72, 0x6f, 0x6f, 0x6d, 0x2e, 0x4a,
	0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x12, 0x2f, 0x0a, 0x03, 0x53, 0x61, 0x79, 0x12, 0x10, 0x2e, 0x72, 0x6f, 0x6f, 0x6d,
	0x2e, 0x53, 0x61, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x12, 0x37, 0x0a, 0x05, 0x4c, 0x65, 0x61, 0x76, 0x65, 0x12, 0x16, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x04, 0xc0, 0xf3,
	0x18, 0x01, 0x3a, 0x59, 0x0a, 0x0d, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x5f, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x12, 0x1f, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4f, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe0, 0xd4, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x63,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x52,
	0x0c, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x3a, 0x51, 0x0a,
	0x0a, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x1f, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe1, 0xd4, 0x03,
	0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x72, 0x6f, 0x6f, 0x6d, 0x2e, 0x52, 0x65, 0x70, 0x6c,
	0x79, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x09, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65,
	0x42, 0x37, 0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a,
	0x6f, 0x79, 0x70, 0x61, 0x72, 0x74, 0x79, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f,
	0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x72, 0x6f, 0x6f, 0x6d, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_room_service_proto_rawDescOnce sync.Once
	file_room_service_proto_rawDescData = file_room_service_proto_rawDesc
)

func file_room_service_proto_rawDescGZIP() []byte {
	file_room_service_proto_rawDescOnce.Do(func() {
		file_room_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_room_service_proto_rawDescData)
	})
	return file_room_service_proto_rawDescData
}

var file_room_service_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_room_service_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_room_service_proto_goTypes = []any{
	(ReplyCode)(0),                      // 0: room.ReplyCode
	(*JoinRequest)(nil),                 // 1: room.JoinRequest
	(*SayRequest)(nil),                  // 2: room.SayRequest
	(*News)(nil),                        // 3: room.News
	(*descriptorpb.MessageOptions)(nil), // 4: google.protobuf.MessageOptions
	(clusterpb.Services)(0),             // 5: cluster.Services
	(*emptypb.Empty)(nil),               // 6: google.protobuf.Empty
}
var file_room_service_proto_depIdxs = []int32{
	4, // 0: room.reply_service:extendee -> google.protobuf.MessageOptions
	4, // 1: room.reply_code:extendee -> google.protobuf.MessageOptions
	5, // 2: room.reply_service:type_name -> cluster.Services
	0, // 3: room.reply_code:type_name -> room.ReplyCode
	1, // 4: room.Room.Join:input_type -> room.JoinRequest
	2, // 5: room.Room.Say:input_type -> room.SayRequest
	6, // 6: room.Room.Leave:input_type -> google.protobuf.Empty
	6, // 7: room.Room.Join:output_type -> google.protobuf.Empty
	6, // 8: room.Room.Say:output_type -> google.protobuf.Empty
	6, // 9: room.Room.Leave:output_type -> google.protobuf.Empty
	7, // [7:10] is the sub-list for method output_type
	4, // [4:7] is the sub-list for method input_type
	2, // [2:4] is the sub-list for extension type_name
	0, // [0:2] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_room_service_proto_init() }
func file_room_service_proto_init() {
	if File_room_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_room_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*JoinRequest); i {
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
		file_room_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*SayRequest); i {
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
		file_room_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*News); i {
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
			RawDescriptor: file_room_service_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 2,
			NumServices:   1,
		},
		GoTypes:           file_room_service_proto_goTypes,
		DependencyIndexes: file_room_service_proto_depIdxs,
		EnumInfos:         file_room_service_proto_enumTypes,
		MessageInfos:      file_room_service_proto_msgTypes,
		ExtensionInfos:    file_room_service_proto_extTypes,
	}.Build()
	File_room_service_proto = out.File
	file_room_service_proto_rawDesc = nil
	file_room_service_proto_goTypes = nil
	file_room_service_proto_depIdxs = nil
}
