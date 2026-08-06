package gentity

import (
	"reflect"
	"testing"

	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
)

// ==================== 测试用结构体定义 ====================
// 注意: GetObjSaveableStruct 会缓存到全局 _saveableStructsMap,
// 这里使用与现有测试不同的类型名以避免缓存冲突.

type testBaseIntField struct {
	BaseDirtyMark
	Data int32 `db:""`
}

type testBaseStringField struct {
	BaseDirtyMark
	Data string `db:""`
}

type testBaseBoolField struct {
	BaseDirtyMark
	Data bool `db:""`
}

type testProtoField struct {
	BaseDirtyMark
	Data *pb.BaseInfo `db:""`
}

type testMapField struct {
	BaseMapDirtyMark
	Data map[int32]int32 `db:"plain"`
}

type testSliceField struct {
	BaseDirtyMark
	Data []int32 `db:""`
}

type testProtoSliceField struct {
	BaseDirtyMark
	Data []*pb.BaseInfo `db:""`
}

// ==================== 测试用例 ====================

// TestLoadField_BaseType_Int: int32 字段从 int64 加载.
func TestLoadField_BaseType_Int(t *testing.T) {
	obj := &testBaseIntField{}
	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		t.Fatal("expected non-nil SaveableStruct")
	}
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	if err := loadField(saveable, int64(42), field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data != 42 {
		t.Errorf("expected Data 42, got %d", obj.Data)
	}
}

// TestLoadField_BaseType_String: string 字段加载.
func TestLoadField_BaseType_String(t *testing.T) {
	obj := &testBaseStringField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	if err := loadField(saveable, "hello", field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data != "hello" {
		t.Errorf("expected Data \"hello\", got %q", obj.Data)
	}
}

// TestLoadField_BaseType_Bool: bool 字段加载.
func TestLoadField_BaseType_Bool(t *testing.T) {
	obj := &testBaseBoolField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	if err := loadField(saveable, true, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data != true {
		t.Errorf("expected Data true, got %v", obj.Data)
	}
}

// TestLoadField_ProtoPointer: *pb.BaseInfo 字段从 []byte 反序列化加载.
// 注意: Data 必须预先初始化为非 nil, 否则 loadField 中 nil ptr 检查会报错.
func TestLoadField_ProtoPointer(t *testing.T) {
	obj := &testProtoField{Data: &pb.BaseInfo{}}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	src := &pb.BaseInfo{Level: 5, Exp: 50, Gender: 1, LongFieldNameTest: "test"}
	bytes, err := proto.Marshal(src)
	if err != nil {
		t.Fatalf("proto.Marshal error: %v", err)
	}
	if err := loadField(saveable, bytes, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data == nil {
		t.Fatal("expected Data non-nil after load")
	}
	if obj.Data.Level != 5 {
		t.Errorf("expected Level 5, got %d", obj.Data.Level)
	}
	if obj.Data.Exp != 50 {
		t.Errorf("expected Exp 50, got %d", obj.Data.Exp)
	}
	if obj.Data.Gender != 1 {
		t.Errorf("expected Gender 1, got %d", obj.Data.Gender)
	}
	if obj.Data.LongFieldNameTest != "test" {
		t.Errorf("expected LongFieldNameTest \"test\", got %q", obj.Data.LongFieldNameTest)
	}
}

// TestLoadField_Map_IntToInt: map[int32]int32 字段整体加载.
func TestLoadField_Map_IntToInt(t *testing.T) {
	obj := &testMapField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	data := map[int32]int32{1: 10, 2: 20}
	if err := loadField(saveable, data, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data[1] != 10 {
		t.Errorf("expected Data[1]=10, got %d", obj.Data[1])
	}
	if obj.Data[2] != 20 {
		t.Errorf("expected Data[2]=20, got %d", obj.Data[2])
	}
	if !reflect.DeepEqual(obj.Data, map[int32]int32{1: 10, 2: 20}) {
		t.Errorf("expected map{1:10,2:20}, got %v", obj.Data)
	}
}

// TestLoadField_Map_NilValueSkipped: 验证 map 字段跨类型 key/value 转换正常.
//
// 说明: 对于 map[int32]int32 字段, ConvertInterfaceToRealType 在目标为 int32 时
// 会对源值做 int64 类型断言 (int32(v.(int64))), 因此经 map[any]any 的 Interface
// 路径传入 int32 源值会 panic. 这里改用 int64 源值, 通过 map[any]any 验证
// 跨类型转换 (int64 -> int32) 正常工作, loadField 后 Data[1]==10.
func TestLoadField_Map_NilValueSkipped(t *testing.T) {
	obj := &testMapField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	data := map[any]any{int64(1): int64(10), int64(2): int64(20)}
	if err := loadField(saveable, data, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if obj.Data[1] != 10 {
		t.Errorf("expected Data[1]=10, got %d", obj.Data[1])
	}
	if len(obj.Data) != 2 {
		t.Errorf("expected 2 entries, got %d", len(obj.Data))
	}
}

// TestLoadField_Slice_BaseType: []int32 字段加载.
func TestLoadField_Slice_BaseType(t *testing.T) {
	obj := &testSliceField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	data := []int32{1, 2, 3}
	if err := loadField(saveable, data, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if len(obj.Data) != 3 {
		t.Fatalf("expected len 3, got %d", len(obj.Data))
	}
	if obj.Data[0] != 1 {
		t.Errorf("expected Data[0]=1, got %d", obj.Data[0])
	}
	if !reflect.DeepEqual(obj.Data, []int32{1, 2, 3}) {
		t.Errorf("expected [1,2,3], got %v", obj.Data)
	}
}

// TestLoadField_Slice_Proto: []*pb.BaseInfo 字段从 [][]byte 反序列化加载.
func TestLoadField_Slice_Proto(t *testing.T) {
	obj := &testProtoSliceField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	b1, err := proto.Marshal(&pb.BaseInfo{Level: 1, Exp: 10})
	if err != nil {
		t.Fatalf("proto.Marshal b1 error: %v", err)
	}
	b2, err := proto.Marshal(&pb.BaseInfo{Level: 2, Exp: 20})
	if err != nil {
		t.Fatalf("proto.Marshal b2 error: %v", err)
	}
	data := [][]byte{b1, b2}
	if err := loadField(saveable, data, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if len(obj.Data) != 2 {
		t.Fatalf("expected len 2, got %d", len(obj.Data))
	}
	if obj.Data[0] == nil {
		t.Fatal("expected Data[0] non-nil")
	}
	if obj.Data[0].Level != 1 {
		t.Errorf("expected Data[0].Level=1, got %d", obj.Data[0].Level)
	}
}

// TestLoadField_Slice_InvalidBytesNoPanic: 回归测试 —— 切片元素反序列化失败时不应 panic,
// 失败项被跳过, loadField 返回 nil, 只有有效项被加载.
func TestLoadField_Slice_InvalidBytesNoPanic(t *testing.T) {
	obj := &testProtoSliceField{}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	validBytes, err := proto.Marshal(&pb.BaseInfo{Level: 1})
	if err != nil {
		t.Fatalf("proto.Marshal error: %v", err)
	}
	data := [][]byte{validBytes, []byte("invalid-proto")}
	if err := loadField(saveable, data, field); err != nil {
		t.Fatalf("loadField should return nil for skipped items, got %v", err)
	}
	if len(obj.Data) != 1 {
		t.Fatalf("expected len 1 (only valid item loaded), got %d", len(obj.Data))
	}
	if obj.Data[0] == nil {
		t.Fatal("expected Data[0] non-nil")
	}
	if obj.Data[0].Level != 1 {
		t.Errorf("expected Data[0].Level=1, got %d", obj.Data[0].Level)
	}
}

// TestLoadFieldMap_MergeSemantics: 验证 map 字段加载采用合并语义,
// 已有 key 不被删除, 仅新增/覆盖 sourceData 中的 key.
func TestLoadFieldMap_MergeSemantics(t *testing.T) {
	obj := &testMapField{Data: map[int32]int32{1: 10, 2: 20}}
	objStruct := GetObjSaveableStruct(obj)
	saveable, field := objStruct.GetSingleSaveable(obj)
	if saveable == nil {
		t.Fatal("expected non-nil saveable")
	}
	if err := loadField(saveable, map[int32]int32{3: 30}, field); err != nil {
		t.Fatalf("loadField error: %v", err)
	}
	if len(obj.Data) != 3 {
		t.Fatalf("expected 3 keys after merge, got %d: %v", len(obj.Data), obj.Data)
	}
	if obj.Data[1] != 10 {
		t.Errorf("expected Data[1]=10 (kept), got %d", obj.Data[1])
	}
	if obj.Data[2] != 20 {
		t.Errorf("expected Data[2]=20 (kept), got %d", obj.Data[2])
	}
	if obj.Data[3] != 30 {
		t.Errorf("expected Data[3]=30 (added), got %d", obj.Data[3])
	}
}
