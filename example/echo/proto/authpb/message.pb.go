// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v3.21.12
// source: auth/message.proto

package authpb

import (
	clusterpb "github.com/joyparty/nodehub/example/echo/proto/clusterpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ReplyCode int32

const (
	ReplyCode_UNSPECIFIED   ReplyCode = 0
	ReplyCode_AUTHORIZE_ACK ReplyCode = 1
)

// Enum value maps for ReplyCode.
var (
	ReplyCode_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "AUTHORIZE_ACK",
	}
	ReplyCode_value = map[string]int32{
		"UNSPECIFIED":   0,
		"AUTHORIZE_ACK": 1,
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
	return file_auth_message_proto_enumTypes[0].Descriptor()
}

func (ReplyCode) Type() protoreflect.EnumType {
	return &file_auth_message_proto_enumTypes[0]
}

func (x ReplyCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

type AuthorizeToken struct {
	state            protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Token string                 `protobuf:"bytes,1,opt,name=token,proto3"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *AuthorizeToken) Reset() {
	*x = AuthorizeToken{}
	mi := &file_auth_message_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AuthorizeToken) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AuthorizeToken) ProtoMessage() {}

func (x *AuthorizeToken) ProtoReflect() protoreflect.Message {
	mi := &file_auth_message_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *AuthorizeToken) GetToken() string {
	if x != nil {
		return x.xxx_hidden_Token
	}
	return ""
}

func (x *AuthorizeToken) SetToken(v string) {
	x.xxx_hidden_Token = v
}

type AuthorizeToken_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Token string
}

func (b0 AuthorizeToken_builder) Build() *AuthorizeToken {
	m0 := &AuthorizeToken{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Token = b.Token
	return m0
}

type AuthorizeAck struct {
	state             protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_UserId string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *AuthorizeAck) Reset() {
	*x = AuthorizeAck{}
	mi := &file_auth_message_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AuthorizeAck) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AuthorizeAck) ProtoMessage() {}

func (x *AuthorizeAck) ProtoReflect() protoreflect.Message {
	mi := &file_auth_message_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *AuthorizeAck) GetUserId() string {
	if x != nil {
		return x.xxx_hidden_UserId
	}
	return ""
}

func (x *AuthorizeAck) SetUserId(v string) {
	x.xxx_hidden_UserId = v
}

type AuthorizeAck_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	UserId string
}

func (b0 AuthorizeAck_builder) Build() *AuthorizeAck {
	m0 := &AuthorizeAck{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_UserId = b.UserId
	return m0
}

var file_auth_message_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*clusterpb.Services)(nil),
		Field:         60000,
		Name:          "auth.reply_service",
		Tag:           "varint,60000,opt,name=reply_service,enum=cluster.Services",
		Filename:      "auth/message.proto",
	},
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*ReplyCode)(nil),
		Field:         60001,
		Name:          "auth.reply_code",
		Tag:           "varint,60001,opt,name=reply_code,enum=auth.ReplyCode",
		Filename:      "auth/message.proto",
	},
}

// Extension fields to descriptorpb.MessageOptions.
var (
	// optional cluster.Services reply_service = 60000;
	E_ReplyService = &file_auth_message_proto_extTypes[0]
	// optional auth.ReplyCode reply_code = 60001;
	E_ReplyCode = &file_auth_message_proto_extTypes[1]
)

var File_auth_message_proto protoreflect.FileDescriptor

var file_auth_message_proto_rawDesc = string([]byte{
	0x0a, 0x12, 0x61, 0x75, 0x74, 0x68, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x61, 0x75, 0x74, 0x68, 0x1a, 0x16, 0x63, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x26, 0x0a, 0x0e, 0x41, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x7a,
	0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x31, 0x0a, 0x0c,
	0x41, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x7a, 0x65, 0x41, 0x63, 0x6b, 0x12, 0x17, 0x0a, 0x07,
	0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x75,
	0x73, 0x65, 0x72, 0x49, 0x64, 0x3a, 0x08, 0x80, 0xa6, 0x1d, 0x01, 0x88, 0xa6, 0x1d, 0x01, 0x2a,
	0x2f, 0x0a, 0x09, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x0f, 0x0a, 0x0b,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x11, 0x0a,
	0x0d, 0x41, 0x55, 0x54, 0x48, 0x4f, 0x52, 0x49, 0x5a, 0x45, 0x5f, 0x41, 0x43, 0x4b, 0x10, 0x01,
	0x3a, 0x59, 0x0a, 0x0d, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x12, 0x1f, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x18, 0xe0, 0xd4, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x63, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x52, 0x0c, 0x72,
	0x65, 0x70, 0x6c, 0x79, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x3a, 0x51, 0x0a, 0x0a, 0x72,
	0x65, 0x70, 0x6c, 0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x1f, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe1, 0xd4, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x2e, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x43,
	0x6f, 0x64, 0x65, 0x52, 0x09, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x42, 0x37,
	0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a, 0x6f, 0x79,
	0x70, 0x61, 0x72, 0x74, 0x79, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x65, 0x78,
	0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2f, 0x65, 0x63, 0x68, 0x6f, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var file_auth_message_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_auth_message_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_auth_message_proto_goTypes = []any{
	(ReplyCode)(0),                      // 0: auth.ReplyCode
	(*AuthorizeToken)(nil),              // 1: auth.AuthorizeToken
	(*AuthorizeAck)(nil),                // 2: auth.AuthorizeAck
	(*descriptorpb.MessageOptions)(nil), // 3: google.protobuf.MessageOptions
	(clusterpb.Services)(0),             // 4: cluster.Services
}
var file_auth_message_proto_depIdxs = []int32{
	3, // 0: auth.reply_service:extendee -> google.protobuf.MessageOptions
	3, // 1: auth.reply_code:extendee -> google.protobuf.MessageOptions
	4, // 2: auth.reply_service:type_name -> cluster.Services
	0, // 3: auth.reply_code:type_name -> auth.ReplyCode
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	2, // [2:4] is the sub-list for extension type_name
	0, // [0:2] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_auth_message_proto_init() }
func file_auth_message_proto_init() {
	if File_auth_message_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_auth_message_proto_rawDesc), len(file_auth_message_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 2,
			NumServices:   0,
		},
		GoTypes:           file_auth_message_proto_goTypes,
		DependencyIndexes: file_auth_message_proto_depIdxs,
		EnumInfos:         file_auth_message_proto_enumTypes,
		MessageInfos:      file_auth_message_proto_msgTypes,
		ExtensionInfos:    file_auth_message_proto_extTypes,
	}.Build()
	File_auth_message_proto = out.File
	file_auth_message_proto_goTypes = nil
	file_auth_message_proto_depIdxs = nil
}
