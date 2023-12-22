// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v3.21.12
// source: nodehub/gateway_service.proto

package nodehubpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
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

type SetServiceRouteRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ServiceCode int32  `protobuf:"varint,1,opt,name=service_code,json=serviceCode,proto3" json:"service_code,omitempty"`
	SessionId   string `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	NodeId      string `protobuf:"bytes,3,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
}

func (x *SetServiceRouteRequest) Reset() {
	*x = SetServiceRouteRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetServiceRouteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetServiceRouteRequest) ProtoMessage() {}

func (x *SetServiceRouteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetServiceRouteRequest.ProtoReflect.Descriptor instead.
func (*SetServiceRouteRequest) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_service_proto_rawDescGZIP(), []int{0}
}

func (x *SetServiceRouteRequest) GetServiceCode() int32 {
	if x != nil {
		return x.ServiceCode
	}
	return 0
}

func (x *SetServiceRouteRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *SetServiceRouteRequest) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

type RemoveServiceRouteRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ServiceCode int32  `protobuf:"varint,1,opt,name=service_code,json=serviceCode,proto3" json:"service_code,omitempty"`
	SessionId   string `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
}

func (x *RemoveServiceRouteRequest) Reset() {
	*x = RemoveServiceRouteRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RemoveServiceRouteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RemoveServiceRouteRequest) ProtoMessage() {}

func (x *RemoveServiceRouteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RemoveServiceRouteRequest.ProtoReflect.Descriptor instead.
func (*RemoveServiceRouteRequest) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_service_proto_rawDescGZIP(), []int{1}
}

func (x *RemoveServiceRouteRequest) GetServiceCode() int32 {
	if x != nil {
		return x.ServiceCode
	}
	return 0
}

func (x *RemoveServiceRouteRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type IsSessionExistRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SessionId string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
}

func (x *IsSessionExistRequest) Reset() {
	*x = IsSessionExistRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IsSessionExistRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IsSessionExistRequest) ProtoMessage() {}

func (x *IsSessionExistRequest) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IsSessionExistRequest.ProtoReflect.Descriptor instead.
func (*IsSessionExistRequest) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_service_proto_rawDescGZIP(), []int{2}
}

func (x *IsSessionExistRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type IsSessionExistResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SessionId string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Exist     bool   `protobuf:"varint,2,opt,name=exist,proto3" json:"exist,omitempty"`
}

func (x *IsSessionExistResponse) Reset() {
	*x = IsSessionExistResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_nodehub_gateway_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IsSessionExistResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IsSessionExistResponse) ProtoMessage() {}

func (x *IsSessionExistResponse) ProtoReflect() protoreflect.Message {
	mi := &file_nodehub_gateway_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IsSessionExistResponse.ProtoReflect.Descriptor instead.
func (*IsSessionExistResponse) Descriptor() ([]byte, []int) {
	return file_nodehub_gateway_service_proto_rawDescGZIP(), []int{3}
}

func (x *IsSessionExistResponse) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *IsSessionExistResponse) GetExist() bool {
	if x != nil {
		return x.Exist
	}
	return false
}

var File_nodehub_gateway_service_proto protoreflect.FileDescriptor

var file_nodehub_gateway_service_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x67, 0x61, 0x74, 0x65, 0x77, 0x61,
	0x79, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x73, 0x0a, 0x16, 0x53, 0x65, 0x74, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x21, 0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f,
	0x64, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49,
	0x64, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x22, 0x5d, 0x0a, 0x19, 0x52, 0x65,
	0x6d, 0x6f, 0x76, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52, 0x6f, 0x75, 0x74, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0b, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09,
	0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x22, 0x36, 0x0a, 0x15, 0x49, 0x73, 0x53,
	0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x45, 0x78, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49,
	0x64, 0x22, 0x4d, 0x0a, 0x16, 0x49, 0x73, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x45, 0x78,
	0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x73,
	0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x09, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x78,
	0x69, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x65, 0x78, 0x69, 0x73, 0x74,
	0x32, 0x80, 0x02, 0x0a, 0x07, 0x47, 0x61, 0x74, 0x65, 0x77, 0x61, 0x79, 0x12, 0x53, 0x0a, 0x0e,
	0x49, 0x73, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x45, 0x78, 0x69, 0x73, 0x74, 0x12, 0x1e,
	0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2e, 0x49, 0x73, 0x53, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x45, 0x78, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1f,
	0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2e, 0x49, 0x73, 0x53, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x45, 0x78, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x4c, 0x0a, 0x0f, 0x53, 0x65, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52,
	0x6f, 0x75, 0x74, 0x65, 0x12, 0x1f, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2e, 0x53,
	0x65, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12,
	0x52, 0x0a, 0x12, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x52, 0x6f, 0x75, 0x74, 0x65, 0x12, 0x22, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2e,
	0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52, 0x6f, 0x75,
	0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x22, 0x00, 0x42, 0x32, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x6c, 0x61, 0x62, 0x2e, 0x68, 0x61,
	0x6f, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x2e, 0x74, 0x76, 0x2f, 0x67, 0x6f, 0x70, 0x6b, 0x67, 0x2f,
	0x6e, 0x6f, 0x64, 0x65, 0x68, 0x75, 0x62, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x6f,
	0x64, 0x65, 0x68, 0x75, 0x62, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_nodehub_gateway_service_proto_rawDescOnce sync.Once
	file_nodehub_gateway_service_proto_rawDescData = file_nodehub_gateway_service_proto_rawDesc
)

func file_nodehub_gateway_service_proto_rawDescGZIP() []byte {
	file_nodehub_gateway_service_proto_rawDescOnce.Do(func() {
		file_nodehub_gateway_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_nodehub_gateway_service_proto_rawDescData)
	})
	return file_nodehub_gateway_service_proto_rawDescData
}

var file_nodehub_gateway_service_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_nodehub_gateway_service_proto_goTypes = []interface{}{
	(*SetServiceRouteRequest)(nil),    // 0: nodehub.SetServiceRouteRequest
	(*RemoveServiceRouteRequest)(nil), // 1: nodehub.RemoveServiceRouteRequest
	(*IsSessionExistRequest)(nil),     // 2: nodehub.IsSessionExistRequest
	(*IsSessionExistResponse)(nil),    // 3: nodehub.IsSessionExistResponse
	(*emptypb.Empty)(nil),             // 4: google.protobuf.Empty
}
var file_nodehub_gateway_service_proto_depIdxs = []int32{
	2, // 0: nodehub.Gateway.IsSessionExist:input_type -> nodehub.IsSessionExistRequest
	0, // 1: nodehub.Gateway.SetServiceRoute:input_type -> nodehub.SetServiceRouteRequest
	1, // 2: nodehub.Gateway.RemoveServiceRoute:input_type -> nodehub.RemoveServiceRouteRequest
	3, // 3: nodehub.Gateway.IsSessionExist:output_type -> nodehub.IsSessionExistResponse
	4, // 4: nodehub.Gateway.SetServiceRoute:output_type -> google.protobuf.Empty
	4, // 5: nodehub.Gateway.RemoveServiceRoute:output_type -> google.protobuf.Empty
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_nodehub_gateway_service_proto_init() }
func file_nodehub_gateway_service_proto_init() {
	if File_nodehub_gateway_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_nodehub_gateway_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetServiceRouteRequest); i {
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
		file_nodehub_gateway_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RemoveServiceRouteRequest); i {
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
		file_nodehub_gateway_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*IsSessionExistRequest); i {
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
		file_nodehub_gateway_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*IsSessionExistResponse); i {
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
			RawDescriptor: file_nodehub_gateway_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_nodehub_gateway_service_proto_goTypes,
		DependencyIndexes: file_nodehub_gateway_service_proto_depIdxs,
		MessageInfos:      file_nodehub_gateway_service_proto_msgTypes,
	}.Build()
	File_nodehub_gateway_service_proto = out.File
	file_nodehub_gateway_service_proto_rawDesc = nil
	file_nodehub_gateway_service_proto_goTypes = nil
	file_nodehub_gateway_service_proto_depIdxs = nil
}
