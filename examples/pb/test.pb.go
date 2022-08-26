// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1-devel
// 	protoc        v3.19.1
// source: test.proto

package pb

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

// 玩家基础信息
type BaseInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Gender int32 `protobuf:"varint,1,opt,name=gender,proto3" json:"gender,omitempty"` // 性别
	Level  int32 `protobuf:"varint,2,opt,name=level,proto3" json:"level,omitempty"`   // 等级
	Exp    int32 `protobuf:"varint,3,opt,name=exp,proto3" json:"exp,omitempty"`       // 经验值
}

func (x *BaseInfo) Reset() {
	*x = BaseInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BaseInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BaseInfo) ProtoMessage() {}

func (x *BaseInfo) ProtoReflect() protoreflect.Message {
	mi := &file_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BaseInfo.ProtoReflect.Descriptor instead.
func (*BaseInfo) Descriptor() ([]byte, []int) {
	return file_test_proto_rawDescGZIP(), []int{0}
}

func (x *BaseInfo) GetGender() int32 {
	if x != nil {
		return x.Gender
	}
	return 0
}

func (x *BaseInfo) GetLevel() int32 {
	if x != nil {
		return x.Level
	}
	return 0
}

func (x *BaseInfo) GetExp() int32 {
	if x != nil {
		return x.Exp
	}
	return 0
}

// 任务模块数据
type QuestSaveData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Finished []int32          `protobuf:"varint,1,rep,packed,name=finished,proto3" json:"finished,omitempty"`                                                                              // 已完成的任务
	Quests   map[int32][]byte `protobuf:"bytes,2,rep,name=quests,proto3" json:"quests,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"` // 进行中的任务
}

func (x *QuestSaveData) Reset() {
	*x = QuestSaveData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QuestSaveData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QuestSaveData) ProtoMessage() {}

func (x *QuestSaveData) ProtoReflect() protoreflect.Message {
	mi := &file_test_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QuestSaveData.ProtoReflect.Descriptor instead.
func (*QuestSaveData) Descriptor() ([]byte, []int) {
	return file_test_proto_rawDescGZIP(), []int{1}
}

func (x *QuestSaveData) GetFinished() []int32 {
	if x != nil {
		return x.Finished
	}
	return nil
}

func (x *QuestSaveData) GetQuests() map[int32][]byte {
	if x != nil {
		return x.Quests
	}
	return nil
}

// 任务数据
type QuestData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CfgId    int32 `protobuf:"varint,1,opt,name=cfgId,proto3" json:"cfgId,omitempty"`       // 配置id
	Progress int32 `protobuf:"varint,2,opt,name=progress,proto3" json:"progress,omitempty"` // 进度
}

func (x *QuestData) Reset() {
	*x = QuestData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QuestData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QuestData) ProtoMessage() {}

func (x *QuestData) ProtoReflect() protoreflect.Message {
	mi := &file_test_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QuestData.ProtoReflect.Descriptor instead.
func (*QuestData) Descriptor() ([]byte, []int) {
	return file_test_proto_rawDescGZIP(), []int{2}
}

func (x *QuestData) GetCfgId() int32 {
	if x != nil {
		return x.CfgId
	}
	return 0
}

func (x *QuestData) GetProgress() int32 {
	if x != nil {
		return x.Progress
	}
	return 0
}

// 玩家在mongo中的保存格式
// 用于一次性把玩家数据加载进来
type PlayerData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id        int64          `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`               // 玩家id
	Name      string         `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`            // 玩家名
	AccountId int64          `protobuf:"varint,3,opt,name=accountId,proto3" json:"accountId,omitempty"` // 账号id
	RegionId  int32          `protobuf:"varint,4,opt,name=regionId,proto3" json:"regionId,omitempty"`   // 区服id
	BaseInfo  *BaseInfo      `protobuf:"bytes,5,opt,name=baseInfo,proto3" json:"baseInfo,omitempty"`
	Quest     *QuestSaveData `protobuf:"bytes,8,opt,name=quest,proto3" json:"quest,omitempty"`
}

func (x *PlayerData) Reset() {
	*x = PlayerData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlayerData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlayerData) ProtoMessage() {}

func (x *PlayerData) ProtoReflect() protoreflect.Message {
	mi := &file_test_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlayerData.ProtoReflect.Descriptor instead.
func (*PlayerData) Descriptor() ([]byte, []int) {
	return file_test_proto_rawDescGZIP(), []int{3}
}

func (x *PlayerData) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *PlayerData) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *PlayerData) GetAccountId() int64 {
	if x != nil {
		return x.AccountId
	}
	return 0
}

func (x *PlayerData) GetRegionId() int32 {
	if x != nil {
		return x.RegionId
	}
	return 0
}

func (x *PlayerData) GetBaseInfo() *BaseInfo {
	if x != nil {
		return x.BaseInfo
	}
	return nil
}

func (x *PlayerData) GetQuest() *QuestSaveData {
	if x != nil {
		return x.Quest
	}
	return nil
}

var File_test_proto protoreflect.FileDescriptor

var file_test_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x67, 0x73,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x22, 0x4a, 0x0a, 0x08, 0x42, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x66,
	0x6f, 0x12, 0x16, 0x0a, 0x06, 0x67, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x05, 0x52, 0x06, 0x67, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x65, 0x76,
	0x65, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x12,
	0x10, 0x0a, 0x03, 0x65, 0x78, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x65, 0x78,
	0x70, 0x22, 0xa2, 0x01, 0x0a, 0x0d, 0x51, 0x75, 0x65, 0x73, 0x74, 0x53, 0x61, 0x76, 0x65, 0x44,
	0x61, 0x74, 0x61, 0x12, 0x1a, 0x0a, 0x08, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x05, 0x52, 0x08, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x12,
	0x3a, 0x0a, 0x06, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x22, 0x2e, 0x67, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x73, 0x74, 0x53,
	0x61, 0x76, 0x65, 0x44, 0x61, 0x74, 0x61, 0x2e, 0x51, 0x75, 0x65, 0x73, 0x74, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x52, 0x06, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x1a, 0x39, 0x0a, 0x0b, 0x51,
	0x75, 0x65, 0x73, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x3d, 0x0a, 0x09, 0x51, 0x75, 0x65, 0x73, 0x74, 0x44,
	0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x66, 0x67, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x05, 0x63, 0x66, 0x67, 0x49, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f,
	0x67, 0x72, 0x65, 0x73, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x70, 0x72, 0x6f,
	0x67, 0x72, 0x65, 0x73, 0x73, 0x22, 0xc7, 0x01, 0x0a, 0x0a, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72,
	0x44, 0x61, 0x74, 0x61, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x61, 0x63, 0x63, 0x6f,
	0x75, 0x6e, 0x74, 0x49, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x61, 0x63, 0x63,
	0x6f, 0x75, 0x6e, 0x74, 0x49, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e,
	0x49, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e,
	0x49, 0x64, 0x12, 0x2d, 0x0a, 0x08, 0x62, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x67, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x42,
	0x61, 0x73, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08, 0x62, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x66,
	0x6f, 0x12, 0x2c, 0x0a, 0x05, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x16, 0x2e, 0x67, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x73, 0x74,
	0x53, 0x61, 0x76, 0x65, 0x44, 0x61, 0x74, 0x61, 0x52, 0x05, 0x71, 0x75, 0x65, 0x73, 0x74, 0x42,
	0x06, 0x5a, 0x04, 0x2e, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_test_proto_rawDescOnce sync.Once
	file_test_proto_rawDescData = file_test_proto_rawDesc
)

func file_test_proto_rawDescGZIP() []byte {
	file_test_proto_rawDescOnce.Do(func() {
		file_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_test_proto_rawDescData)
	})
	return file_test_proto_rawDescData
}

var file_test_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_test_proto_goTypes = []interface{}{
	(*BaseInfo)(nil),      // 0: gserver.BaseInfo
	(*QuestSaveData)(nil), // 1: gserver.QuestSaveData
	(*QuestData)(nil),     // 2: gserver.QuestData
	(*PlayerData)(nil),    // 3: gserver.PlayerData
	nil,                   // 4: gserver.QuestSaveData.QuestsEntry
}
var file_test_proto_depIdxs = []int32{
	4, // 0: gserver.QuestSaveData.quests:type_name -> gserver.QuestSaveData.QuestsEntry
	0, // 1: gserver.PlayerData.baseInfo:type_name -> gserver.BaseInfo
	1, // 2: gserver.PlayerData.quest:type_name -> gserver.QuestSaveData
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_test_proto_init() }
func file_test_proto_init() {
	if File_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_test_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BaseInfo); i {
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
		file_test_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QuestSaveData); i {
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
		file_test_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QuestData); i {
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
		file_test_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PlayerData); i {
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
			RawDescriptor: file_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_test_proto_goTypes,
		DependencyIndexes: file_test_proto_depIdxs,
		MessageInfos:      file_test_proto_msgTypes,
	}.Build()
	File_test_proto = out.File
	file_test_proto_rawDesc = nil
	file_test_proto_goTypes = nil
	file_test_proto_depIdxs = nil
}
