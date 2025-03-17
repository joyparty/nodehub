// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v3.21.12
// source: echo/service.proto

package echopb

import (
	_ "github.com/joyparty/nodehub/example/echo/proto/clusterpb"
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
	ReplyCode_UNSPECIFIED ReplyCode = 0
	ReplyCode_MSG         ReplyCode = 1
)

// Enum value maps for ReplyCode.
var (
	ReplyCode_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "MSG",
	}
	ReplyCode_value = map[string]int32{
		"UNSPECIFIED": 0,
		"MSG":         1,
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
	return file_echo_service_proto_enumTypes[0].Descriptor()
}

func (ReplyCode) Type() protoreflect.EnumType {
	return &file_echo_service_proto_enumTypes[0]
}

func (x ReplyCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

type Msg struct {
	state              protoimpl.MessageState `protogen:"opaque.v1"`
	xxx_hidden_Content string                 `protobuf:"bytes,1,opt,name=content,proto3"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *Msg) Reset() {
	*x = Msg{}
	mi := &file_echo_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Msg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Msg) ProtoMessage() {}

func (x *Msg) ProtoReflect() protoreflect.Message {
	mi := &file_echo_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *Msg) GetContent() string {
	if x != nil {
		return x.xxx_hidden_Content
	}
	return ""
}

func (x *Msg) SetContent(v string) {
	x.xxx_hidden_Content = v
}

type Msg_builder struct {
	_ [0]func() // Prevents comparability and use of unkeyed literals for the builder.

	Content string
}

func (b0 Msg_builder) Build() *Msg {
	m0 := &Msg{}
	b, x := &b0, m0
	_, _ = b, x
	x.xxx_hidden_Content = b.Content
	return m0
}

var file_echo_service_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.MessageOptions)(nil),
		ExtensionType: (*ReplyCode)(nil),
		Field:         60002,
		Name:          "echo.reply_code",
		Tag:           "varint,60002,opt,name=reply_code,enum=echo.ReplyCode",
		Filename:      "echo/service.proto",
	},
}

// Extension fields to descriptorpb.MessageOptions.
var (
	// optional echo.ReplyCode reply_code = 60002;
	E_ReplyCode = &file_echo_service_proto_extTypes[0]
)

var File_echo_service_proto protoreflect.FileDescriptor

var file_echo_service_proto_rawDesc = string([]byte{
	0x0a, 0x12, 0x65, 0x63, 0x68, 0x6f, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x65, 0x63, 0x68, 0x6f, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x16, 0x63, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x25, 0x0a, 0x03, 0x4d, 0x73, 0x67, 0x12, 0x18, 0x0a, 0x07, 0x63,
	0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x3a, 0x04, 0x90, 0xa6, 0x1d, 0x01, 0x2a, 0x25, 0x0a, 0x09, 0x52,
	0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x0f, 0x0a, 0x0b, 0x55, 0x4e, 0x53, 0x50,
	0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x07, 0x0a, 0x03, 0x4d, 0x53, 0x47,
	0x10, 0x01, 0x32, 0x2a, 0x0a, 0x04, 0x45, 0x63, 0x68, 0x6f, 0x12, 0x1c, 0x0a, 0x04, 0x53, 0x65,
	0x6e, 0x64, 0x12, 0x09, 0x2e, 0x65, 0x63, 0x68, 0x6f, 0x2e, 0x4d, 0x73, 0x67, 0x1a, 0x09, 0x2e,
	0x65, 0x63, 0x68, 0x6f, 0x2e, 0x4d, 0x73, 0x67, 0x1a, 0x04, 0xc0, 0xf3, 0x18, 0x02, 0x3a, 0x51,
	0x0a, 0x0a, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x1f, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xe2, 0xd4,
	0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x65, 0x63, 0x68, 0x6f, 0x2e, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x09, 0x72, 0x65, 0x70, 0x6c, 0x79, 0x43, 0x6f, 0x64,
	0x65, 0x42, 0x37, 0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6a, 0x6f, 0x79, 0x70, 0x61, 0x72, 0x74, 0x79, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62,
	0x2f, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2f, 0x65, 0x63, 0x68, 0x6f, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x63, 0x68, 0x6f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
})

var file_echo_service_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_echo_service_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_echo_service_proto_goTypes = []any{
	(ReplyCode)(0),                      // 0: echo.ReplyCode
	(*Msg)(nil),                         // 1: echo.Msg
	(*descriptorpb.MessageOptions)(nil), // 2: google.protobuf.MessageOptions
}
var file_echo_service_proto_depIdxs = []int32{
	2, // 0: echo.reply_code:extendee -> google.protobuf.MessageOptions
	0, // 1: echo.reply_code:type_name -> echo.ReplyCode
	1, // 2: echo.Echo.Send:input_type -> echo.Msg
	1, // 3: echo.Echo.Send:output_type -> echo.Msg
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	1, // [1:2] is the sub-list for extension type_name
	0, // [0:1] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_echo_service_proto_init() }
func file_echo_service_proto_init() {
	if File_echo_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_echo_service_proto_rawDesc), len(file_echo_service_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 1,
			NumServices:   1,
		},
		GoTypes:           file_echo_service_proto_goTypes,
		DependencyIndexes: file_echo_service_proto_depIdxs,
		EnumInfos:         file_echo_service_proto_enumTypes,
		MessageInfos:      file_echo_service_proto_msgTypes,
		ExtensionInfos:    file_echo_service_proto_extTypes,
	}.Build()
	File_echo_service_proto = out.File
	file_echo_service_proto_goTypes = nil
	file_echo_service_proto_depIdxs = nil
}
