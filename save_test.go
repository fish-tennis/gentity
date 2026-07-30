package gentity

import (
	"testing"

	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
)

// ==================== 测试用结构体定义 ====================
// 注意: GetObjSaveableStruct 会缓存到全局 _saveableStructsMap,
// 不同测试用不同结构体类型名避免冲突.

// proto指针保存
type testProtoSave struct {
	BaseDirtyMark
	Data *pb.BaseInfo `db:""`
}

// 明文保存
type testPlainSave struct {
	BaseDirtyMark
	Data *pb.BaseInfo `db:"plain"`
}

// MapData保存
type testMapSave struct {
	*MapData[int32, *pb.BaseInfo] `db:""`
}

// SliceData保存
type testSliceSave struct {
	BaseDirtyMark
	Data []*pb.BaseInfo `db:""`
}

// child字段组合保存
type testChildSave struct {
	*BaseComponent
	Map1 *MapData[int32, *pb.BaseInfo] `child:"Map1"`
	Map2 *MapData[int32, *pb.BaseInfo] `child:"Map2"`
}

// 无保存字段
type testNoSave struct {
	Name string
	Age  int
}

// ==================== 测试用例 ====================

// TestGetSaveData_ProtoPointer: Data 字段是 *pb.BaseInfo, GetSaveData 返回 []byte
func TestGetSaveData_ProtoPointer(t *testing.T) {
	obj := &testProtoSave{
		Data: &pb.BaseInfo{
			Level:             10,
			Exp:               100,
			LongFieldNameTest: "hello",
		},
	}
	obj.SetDirty()

	saveData, err := GetSaveData(obj, "test")
	if err != nil {
		t.Fatalf("GetSaveData error: %v", err)
	}
	if saveData == nil {
		t.Fatal("GetSaveData returned nil")
	}

	bytes, ok := saveData.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", saveData)
	}

	var info pb.BaseInfo
	if err := proto.Unmarshal(bytes, &info); err != nil {
		t.Fatalf("proto.Unmarshal error: %v", err)
	}
	if info.Level != 10 {
		t.Errorf("expected Level 10, got %d", info.Level)
	}
	if info.Exp != 100 {
		t.Errorf("expected Exp 100, got %d", info.Exp)
	}
	if info.LongFieldNameTest != "hello" {
		t.Errorf("expected LongFieldNameTest 'hello', got %q", info.LongFieldNameTest)
	}
}

// TestGetSaveData_PlainField: Data 字段带 plain tag, 返回原始的 *pb.BaseInfo
func TestGetSaveData_PlainField(t *testing.T) {
	obj := &testPlainSave{
		Data: &pb.BaseInfo{
			Level: 20,
			Exp:   200,
		},
	}
	obj.SetDirty()

	objStruct := GetObjSaveableStruct(obj)
	saveable, saveableField := objStruct.GetSingleSaveable(obj)
	saveData, err := getSaveDataOfSaveable(saveable, saveableField, "test")
	if err != nil {
		t.Fatalf("getSaveDataOfSaveable error: %v", err)
	}

	info, ok := saveData.(*pb.BaseInfo)
	if !ok {
		t.Fatalf("expected *pb.BaseInfo, got %T", saveData)
	}
	if info.Level != 20 {
		t.Errorf("expected Level 20, got %d", info.Level)
	}
	if info.Exp != 200 {
		t.Errorf("expected Exp 200, got %d", info.Exp)
	}
}

// TestGetSaveData_MapData: 使用 MapData, GetSaveData 返回 map (值是[]byte)
func TestGetSaveData_MapData(t *testing.T) {
	obj := &testMapSave{
		MapData: NewMapData[int32, *pb.BaseInfo](),
	}
	obj.Set(1, &pb.BaseInfo{Level: 1})
	obj.Set(2, &pb.BaseInfo{Level: 2})

	saveData, err := GetSaveData(obj, "test")
	if err != nil {
		t.Fatalf("GetSaveData error: %v", err)
	}
	if saveData == nil {
		t.Fatal("GetSaveData returned nil")
	}

	// int32 key 会被 saveFieldMap 转为 int64
	m, ok := saveData.(map[int64]any)
	if !ok {
		t.Fatalf("expected map[int64]any, got %T", saveData)
	}
	if len(m) != 2 {
		t.Fatalf("expected map len 2, got %d", len(m))
	}

	for k, v := range m {
		bytes, ok := v.([]byte)
		if !ok {
			t.Fatalf("key %v: expected []byte value, got %T", k, v)
		}
		var info pb.BaseInfo
		if err := proto.Unmarshal(bytes, &info); err != nil {
			t.Fatalf("key %v: proto.Unmarshal error: %v", k, err)
		}
		// key 1 -> Level 1, key 2 -> Level 2
		if info.Level != int32(k) {
			t.Errorf("key %v: expected Level %d, got %d", k, k, info.Level)
		}
	}
}

// TestGetSaveData_SliceData: 使用 SliceData, getSaveDataOfSaveable 返回 []any (值是[]byte)
func TestGetSaveData_SliceData(t *testing.T) {
	obj := &testSliceSave{
		Data: []*pb.BaseInfo{
			{Level: 1},
			{Level: 2},
		},
	}
	obj.SetDirty()

	objStruct := GetObjSaveableStruct(obj)
	saveable, saveableField := objStruct.GetSingleSaveable(obj)
	saveData, err := getSaveDataOfSaveable(saveable, saveableField, "test")
	if err != nil {
		t.Fatalf("getSaveDataOfSaveable error: %v", err)
	}
	if saveData == nil {
		t.Fatal("getSaveDataOfSaveable returned nil")
	}

	s, ok := saveData.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", saveData)
	}
	if len(s) != 2 {
		t.Fatalf("expected slice len 2, got %d", len(s))
	}

	for i, v := range s {
		bytes, ok := v.([]byte)
		if !ok {
			t.Fatalf("index %d: expected []byte, got %T", i, v)
		}
		var info pb.BaseInfo
		if err := proto.Unmarshal(bytes, &info); err != nil {
			t.Fatalf("index %d: proto.Unmarshal error: %v", i, err)
		}
		if info.Level != int32(i+1) {
			t.Errorf("index %d: expected Level %d, got %d", i, i+1, info.Level)
		}
	}
}

// TestGetSaveData_ChildFields: 带 child tag 的结构, GetSaveData 返回 map[string]any
func TestGetSaveData_ChildFields(t *testing.T) {
	obj := &testChildSave{
		BaseComponent: NewBaseComponent(nil, "test"),
		Map1:          NewMapData[int32, *pb.BaseInfo](),
		Map2:          NewMapData[int32, *pb.BaseInfo](),
	}
	obj.Map1.Set(1, &pb.BaseInfo{Level: 1})
	obj.Map2.Set(2, &pb.BaseInfo{Level: 2})

	saveData, err := GetSaveData(obj, "test")
	if err != nil {
		t.Fatalf("GetSaveData error: %v", err)
	}
	if saveData == nil {
		t.Fatal("GetSaveData returned nil")
	}

	m, ok := saveData.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", saveData)
	}
	if _, ok := m["Map1"]; !ok {
		t.Error("missing key 'Map1'")
	}
	if _, ok := m["Map2"]; !ok {
		t.Error("missing key 'Map2'")
	}
}

// TestGetSaveData_NoSaveableField: 没有 db/child tag 的结构
func TestGetSaveData_NoSaveableField(t *testing.T) {
	obj := &testNoSave{Name: "test", Age: 20}

	objStruct := GetObjSaveableStruct(obj)
	if objStruct != nil {
		t.Fatalf("expected nil SaveableStruct, got %+v", objStruct)
	}

	saveData, err := GetSaveData(obj, "test")
	if err != nil {
		t.Fatalf("GetSaveData error: %v", err)
	}
	if saveData != nil {
		t.Fatalf("expected nil saveData, got %v", saveData)
	}
}

// TestGetObjSaveableStruct_SingleField: 验证单字段结构的解析
func TestGetObjSaveableStruct_SingleField(t *testing.T) {
	obj := &testProtoSave{
		Data: &pb.BaseInfo{Level: 1},
	}
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		t.Fatal("expected non-nil SaveableStruct")
	}
	if objStruct.Field == nil {
		t.Error("expected Field != nil")
	}
	if len(objStruct.Children) != 0 {
		t.Errorf("expected empty Children, got %d", len(objStruct.Children))
	}
	if !objStruct.IsSingleField() {
		t.Error("expected IsSingleField() == true")
	}
}

// TestGetObjSaveableStruct_ChildFields: 验证多child字段结构的解析
func TestGetObjSaveableStruct_ChildFields(t *testing.T) {
	obj := &testChildSave{
		BaseComponent: NewBaseComponent(nil, "test"),
		Map1:          NewMapData[int32, *pb.BaseInfo](),
		Map2:          NewMapData[int32, *pb.BaseInfo](),
	}
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		t.Fatal("expected non-nil SaveableStruct")
	}
	if objStruct.Field != nil {
		t.Error("expected Field == nil")
	}
	if len(objStruct.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(objStruct.Children))
	}
	if objStruct.IsSingleField() {
		t.Error("expected IsSingleField() == false")
	}
}
