// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v3.21.12
// source: cluster/services.proto

package clusterpb

import (
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

type Services int32

const (
	Services_UNSPECIFIED Services = 0
	Services_AUTH        Services = 1
	Services_ECHO        Services = 2
)

// Enum value maps for Services.
var (
	Services_name = map[int32]string{
		0: "UNSPECIFIED",
		1: "AUTH",
		2: "ECHO",
	}
	Services_value = map[string]int32{
		"UNSPECIFIED": 0,
		"AUTH":        1,
		"ECHO":        2,
	}
)

func (x Services) Enum() *Services {
	p := new(Services)
	*p = x
	return p
}

func (x Services) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Services) Descriptor() protoreflect.EnumDescriptor {
	return file_cluster_services_proto_enumTypes[0].Descriptor()
}

func (Services) Type() protoreflect.EnumType {
	return &file_cluster_services_proto_enumTypes[0]
}

func (x Services) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

var file_cluster_services_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.ServiceOptions)(nil),
		ExtensionType: (*Services)(nil),
		Field:         51000,
		Name:          "cluster.service_code",
		Tag:           "varint,51000,opt,name=service_code,enum=cluster.Services",
		Filename:      "cluster/services.proto",
	},
}

// Extension fields to descriptorpb.ServiceOptions.
var (
	// optional cluster.Services service_code = 51000;
	E_ServiceCode = &file_cluster_services_proto_extTypes[0]
)

var File_cluster_services_proto protoreflect.FileDescriptor

var file_cluster_services_proto_rawDesc = string([]byte{
	0x0a, 0x16, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2a, 0x2f, 0x0a, 0x08, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x12,
	0x0f, 0x0a, 0x0b, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00,
	0x12, 0x08, 0x0a, 0x04, 0x41, 0x55, 0x54, 0x48, 0x10, 0x01, 0x12, 0x08, 0x0a, 0x04, 0x45, 0x43,
	0x48, 0x4f, 0x10, 0x02, 0x3a, 0x57, 0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f,
	0x63, 0x6f, 0x64, 0x65, 0x12, 0x1f, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xb8, 0x8e, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e,
	0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73,
	0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x64, 0x65, 0x42, 0x3a, 0x5a,
	0x38, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a, 0x6f, 0x79, 0x70,
	0x61, 0x72, 0x74, 0x79, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x65, 0x78, 0x61,
	0x6d, 0x70, 0x6c, 0x65, 0x2f, 0x65, 0x63, 0x68, 0x6f, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
})

var file_cluster_services_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_cluster_services_proto_goTypes = []any{
	(Services)(0),                       // 0: cluster.Services
	(*descriptorpb.ServiceOptions)(nil), // 1: google.protobuf.ServiceOptions
}
var file_cluster_services_proto_depIdxs = []int32{
	1, // 0: cluster.service_code:extendee -> google.protobuf.ServiceOptions
	0, // 1: cluster.service_code:type_name -> cluster.Services
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	1, // [1:2] is the sub-list for extension type_name
	0, // [0:1] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_cluster_services_proto_init() }
func file_cluster_services_proto_init() {
	if File_cluster_services_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_cluster_services_proto_rawDesc), len(file_cluster_services_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   0,
			NumExtensions: 1,
			NumServices:   0,
		},
		GoTypes:           file_cluster_services_proto_goTypes,
		DependencyIndexes: file_cluster_services_proto_depIdxs,
		EnumInfos:         file_cluster_services_proto_enumTypes,
		ExtensionInfos:    file_cluster_services_proto_extTypes,
	}.Build()
	File_cluster_services_proto = out.File
	file_cluster_services_proto_goTypes = nil
	file_cluster_services_proto_depIdxs = nil
}
