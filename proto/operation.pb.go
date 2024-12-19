// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v3.17.3
// source: operation.proto

package proto

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

type OperationType int32

const (
	OperationType_SET    OperationType = 0
	OperationType_DELETE OperationType = 1 // Add more operation types as needed
)

// Enum value maps for OperationType.
var (
	OperationType_name = map[int32]string{
		0: "SET",
		1: "DELETE",
	}
	OperationType_value = map[string]int32{
		"SET":    0,
		"DELETE": 1,
	}
)

func (x OperationType) Enum() *OperationType {
	p := new(OperationType)
	*p = x
	return p
}

func (x OperationType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (OperationType) Descriptor() protoreflect.EnumDescriptor {
	return file_operation_proto_enumTypes[0].Descriptor()
}

func (OperationType) Type() protoreflect.EnumType {
	return &file_operation_proto_enumTypes[0]
}

func (x OperationType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use OperationType.Descriptor instead.
func (OperationType) EnumDescriptor() ([]byte, []int) {
	return file_operation_proto_rawDescGZIP(), []int{0}
}

type Operation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OperationId string        `protobuf:"bytes,1,opt,name=operation_id,json=operationId,proto3" json:"operation_id,omitempty"`
	Timestamp   int64         `protobuf:"varint,2,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	ReplicaId   string        `protobuf:"bytes,3,opt,name=replica_id,json=replicaId,proto3" json:"replica_id,omitempty"`
	Command     string        `protobuf:"bytes,4,opt,name=command,proto3" json:"command,omitempty"`
	Args        []string      `protobuf:"bytes,5,rep,name=args,proto3" json:"args,omitempty"`
	Type        OperationType `protobuf:"varint,6,opt,name=type,proto3,enum=proto.OperationType" json:"type,omitempty"`
}

func (x *Operation) Reset() {
	*x = Operation{}
	mi := &file_operation_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Operation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Operation) ProtoMessage() {}

func (x *Operation) ProtoReflect() protoreflect.Message {
	mi := &file_operation_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Operation.ProtoReflect.Descriptor instead.
func (*Operation) Descriptor() ([]byte, []int) {
	return file_operation_proto_rawDescGZIP(), []int{0}
}

func (x *Operation) GetOperationId() string {
	if x != nil {
		return x.OperationId
	}
	return ""
}

func (x *Operation) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Operation) GetReplicaId() string {
	if x != nil {
		return x.ReplicaId
	}
	return ""
}

func (x *Operation) GetCommand() string {
	if x != nil {
		return x.Command
	}
	return ""
}

func (x *Operation) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *Operation) GetType() OperationType {
	if x != nil {
		return x.Type
	}
	return OperationType_SET
}

type OperationBatch struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Operations []*Operation `protobuf:"bytes,1,rep,name=operations,proto3" json:"operations,omitempty"`
}

func (x *OperationBatch) Reset() {
	*x = OperationBatch{}
	mi := &file_operation_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *OperationBatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OperationBatch) ProtoMessage() {}

func (x *OperationBatch) ProtoReflect() protoreflect.Message {
	mi := &file_operation_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OperationBatch.ProtoReflect.Descriptor instead.
func (*OperationBatch) Descriptor() ([]byte, []int) {
	return file_operation_proto_rawDescGZIP(), []int{1}
}

func (x *OperationBatch) GetOperations() []*Operation {
	if x != nil {
		return x.Operations
	}
	return nil
}

var File_operation_proto protoreflect.FileDescriptor

var file_operation_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x05, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc3, 0x01, 0x0a, 0x09, 0x4f, 0x70, 0x65,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x21, 0x0a, 0x0c, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x6f, 0x70,
	0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1d, 0x0a, 0x0a, 0x72, 0x65, 0x70, 0x6c, 0x69,
	0x63, 0x61, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x72, 0x65, 0x70,
	0x6c, 0x69, 0x63, 0x61, 0x49, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e,
	0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04,
	0x61, 0x72, 0x67, 0x73, 0x12, 0x28, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x14, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x42,
	0x0a, 0x0e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x61, 0x74, 0x63, 0x68,
	0x12, 0x30, 0x0a, 0x0a, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4f, 0x70, 0x65,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0a, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x2a, 0x24, 0x0a, 0x0d, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x07, 0x0a, 0x03, 0x53, 0x45, 0x54, 0x10, 0x00, 0x12, 0x0a, 0x0a, 0x06,
	0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x10, 0x01, 0x42, 0x09, 0x5a, 0x07, 0x2e, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_operation_proto_rawDescOnce sync.Once
	file_operation_proto_rawDescData = file_operation_proto_rawDesc
)

func file_operation_proto_rawDescGZIP() []byte {
	file_operation_proto_rawDescOnce.Do(func() {
		file_operation_proto_rawDescData = protoimpl.X.CompressGZIP(file_operation_proto_rawDescData)
	})
	return file_operation_proto_rawDescData
}

var file_operation_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_operation_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_operation_proto_goTypes = []any{
	(OperationType)(0),     // 0: proto.OperationType
	(*Operation)(nil),      // 1: proto.Operation
	(*OperationBatch)(nil), // 2: proto.OperationBatch
}
var file_operation_proto_depIdxs = []int32{
	0, // 0: proto.Operation.type:type_name -> proto.OperationType
	1, // 1: proto.OperationBatch.operations:type_name -> proto.Operation
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_operation_proto_init() }
func file_operation_proto_init() {
	if File_operation_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_operation_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_operation_proto_goTypes,
		DependencyIndexes: file_operation_proto_depIdxs,
		EnumInfos:         file_operation_proto_enumTypes,
		MessageInfos:      file_operation_proto_msgTypes,
	}.Build()
	File_operation_proto = out.File
	file_operation_proto_rawDesc = nil
	file_operation_proto_goTypes = nil
	file_operation_proto_depIdxs = nil
}