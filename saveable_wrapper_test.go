package gentity

import (
	"testing"

	"github.com/fish-tennis/gentity/examples/pb"
)

// ==================== BaseDirtyMark 测试 ====================

func TestBaseDirtyMark(t *testing.T) {
	d := &BaseDirtyMark{}
	// 初始状态
	if d.IsDirty() {
		t.Fatalf("初始状态 IsDirty 应为 false")
	}
	if d.IsChanged() {
		t.Fatalf("初始状态 IsChanged 应为 false")
	}

	// SetDirty 后 IsDirty=true && IsChanged=true
	d.SetDirty()
	if !d.IsDirty() {
		t.Fatalf("SetDirty 后 IsDirty 应为 true")
	}
	if !d.IsChanged() {
		t.Fatalf("SetDirty 后 IsChanged 应为 true")
	}

	// ResetDirty 后 IsDirty=false, IsChanged 仍=true
	d.ResetDirty()
	if d.IsDirty() {
		t.Fatalf("ResetDirty 后 IsDirty 应为 false")
	}
	if !d.IsChanged() {
		t.Fatalf("ResetDirty 后 IsChanged 应仍为 true")
	}

	// ResetChanged 后 IsChanged=false
	d.ResetChanged()
	if d.IsChanged() {
		t.Fatalf("ResetChanged 后 IsChanged 应为 false")
	}
}

// ==================== BaseMapDirtyMark 测试 ====================

func TestBaseMapDirtyMark(t *testing.T) {
	m := &BaseMapDirtyMark{}
	// 初始状态
	if m.IsDirty() {
		t.Fatalf("初始状态 IsDirty 应为 false")
	}
	if m.IsChanged() {
		t.Fatalf("初始状态 IsChanged 应为 false")
	}

	// SetDirty(k1,true) 后 IsDirty=true, IsChanged=true
	m.SetDirty("k1", true)
	if !m.IsDirty() {
		t.Fatalf("SetDirty(k1,true) 后 IsDirty 应为 true")
	}
	if !m.IsChanged() {
		t.Fatalf("SetDirty(k1,true) 后 IsChanged 应为 true")
	}

	// SetDirty(k2,true) 后 dirtyMap 有 2 个 key
	m.SetDirty("k2", true)
	if len(m.dirtyMap) != 2 {
		t.Fatalf("SetDirty(k2,true) 后 dirtyMap 长度应为 2, 实际 %d", len(m.dirtyMap))
	}

	// RangeDirtyMap 遍历所有 dirty key
	got := make(map[interface{}]bool)
	m.RangeDirtyMap(func(dirtyKey interface{}, isAddOrUpdate bool) {
		got[dirtyKey] = isAddOrUpdate
	})
	if len(got) != 2 {
		t.Fatalf("RangeDirtyMap 应遍历到 2 个 key, 实际 %d", len(got))
	}
	if v, ok := got["k1"]; !ok || !v {
		t.Fatalf("RangeDirtyMap 应遍历到 k1=true")
	}
	if v, ok := got["k2"]; !ok || !v {
		t.Fatalf("RangeDirtyMap 应遍历到 k2=true")
	}

	// ResetDirty 后 dirtyMap 被重建(空), IsDirty=false
	m.ResetDirty()
	if m.IsDirty() {
		t.Fatalf("ResetDirty 后 IsDirty 应为 false")
	}
	if len(m.dirtyMap) != 0 {
		t.Fatalf("ResetDirty 后 dirtyMap 应为空, 实际长度 %d", len(m.dirtyMap))
	}

	// ResetChanged 后 IsChanged=false
	if !m.IsChanged() {
		t.Fatalf("ResetChanged 前 IsChanged 应仍为 true")
	}
	m.ResetChanged()
	if m.IsChanged() {
		t.Fatalf("ResetChanged 后 IsChanged 应为 false")
	}

	// HasCached/SetCached 测试
	if m.HasCached() {
		t.Fatalf("初始状态 HasCached 应为 false")
	}
	m.SetCached()
	if !m.HasCached() {
		t.Fatalf("SetCached 后 HasCached 应为 true")
	}
}

// ==================== MapData 测试 ====================

func TestMapData(t *testing.T) {
	// Init() 初始化 map (使用零值结构体验证 Init)
	md := &MapData[int32, *pb.BaseInfo]{}
	if md.Data != nil {
		t.Fatalf("Init 前 Data 应为 nil")
	}
	md.Init()
	if md.Data == nil {
		t.Fatalf("Init 后 Data 不应为 nil")
	}

	// Set(k,v) 后 Data[k]==v 且 IsDirty()==true（dirtyMap 有 k）
	md.Set(1, &pb.BaseInfo{Gender: 1, Level: 10})
	if md.Data[1].GetLevel() != 10 {
		t.Fatalf("Set 后 Data[1].Level 应为 10, 实际 %d", md.Data[1].GetLevel())
	}
	if !md.IsDirty() {
		t.Fatalf("Set 后 IsDirty 应为 true")
	}
	v, exists := md.dirtyMap[int32(1)]
	if !exists {
		t.Fatalf("Set 后 dirtyMap 应包含 key 1")
	}
	if !v {
		t.Fatalf("Set 后 dirtyMap[key] 应为 true")
	}

	// Get(k) 返回正确值和 true
	got, ok := md.Get(1)
	if !ok {
		t.Fatalf("Get(1) 应返回 ok=true")
	}
	if got.GetLevel() != 10 {
		t.Fatalf("Get(1) 应返回 Level=10, 实际 %d", got.GetLevel())
	}

	// Contains(k) 返回 true
	if !md.Contains(1) {
		t.Fatalf("Set 后 Contains(1) 应为 true")
	}

	// Range 遍历所有元素
	rangeCount := 0
	md.Range(func(k int32, v *pb.BaseInfo) bool {
		rangeCount++
		return true
	})
	if rangeCount != 1 {
		t.Fatalf("Range 应遍历到 1 个元素, 实际 %d", rangeCount)
	}

	// Delete(k) 后 Contains(k)==false，且 dirtyMap 中 k 的值为 false(表示删除)
	md.Delete(1)
	if md.Contains(1) {
		t.Fatalf("Delete 后 Contains(1) 应为 false")
	}
	v2, exists2 := md.dirtyMap[int32(1)]
	if !exists2 {
		t.Fatalf("Delete 后 dirtyMap 应包含 key 1")
	}
	if v2 {
		t.Fatalf("Delete 后 dirtyMap[key] 应为 false")
	}

	// IsChanged()==true（Set 后）
	if !md.IsChanged() {
		t.Fatalf("操作后 IsChanged 应为 true")
	}

	// ResetChanged() 后 IsChanged()==false
	md.ResetChanged()
	if md.IsChanged() {
		t.Fatalf("ResetChanged 后 IsChanged 应为 false")
	}
}

// ==================== SliceData 测试 ====================

func TestSliceData(t *testing.T) {
	sd := &SliceData[int32]{}

	// Add(1,2,3) 后 len(Data)==3 且 IsDirty()==true && IsChanged()==true
	sd.Add(1, 2, 3)
	if len(sd.Data) != 3 {
		t.Fatalf("Add(1,2,3) 后 len(Data) 应为 3, 实际 %d", len(sd.Data))
	}
	if !sd.IsDirty() {
		t.Fatalf("Add 后 IsDirty 应为 true")
	}
	if !sd.IsChanged() {
		t.Fatalf("Add 后 IsChanged 应为 true")
	}

	// Add() 空参数什么都不做
	prevLen := len(sd.Data)
	sd.Add()
	if len(sd.Data) != prevLen {
		t.Fatalf("Add() 空参数后 len(Data) 应不变, 实际 %d", len(sd.Data))
	}

	// Delete(0,2) 删除前2个元素
	sd.Delete(0, 2)
	if len(sd.Data) != 1 {
		t.Fatalf("Delete(0,2) 后 len(Data) 应为 1, 实际 %d", len(sd.Data))
	}
	if sd.Data[0] != 3 {
		t.Fatalf("Delete(0,2) 后 Data[0] 应为 3, 实际 %d", sd.Data[0])
	}

	// Delete(越界参数) 安全处理不 panic
	sd.Delete(10, 20) // i > len, j > len, clamp 后 i > j, 直接返回
	if len(sd.Data) != 1 {
		t.Fatalf("Delete(10,20) 后 len(Data) 应仍为 1, 实际 %d", len(sd.Data))
	}
	sd.Delete(-1, 1) // i < 0, 直接返回
	if len(sd.Data) != 1 {
		t.Fatalf("Delete(-1,1) 后 len(Data) 应仍为 1, 实际 %d", len(sd.Data))
	}
}

// ==================== MapValueDirtyMark 测试 ====================

func TestMapValueDirtyMark(t *testing.T) {
	parent := &BaseMapDirtyMark{}
	mvd := NewMapValueDirtyMark[int32](parent, 1)

	// SetDirty() 委托给 parent.SetDirty(key, true)
	mvd.SetDirty()
	if !parent.IsDirty() {
		t.Fatalf("mvd.SetDirty() 后 parent.IsDirty 应为 true")
	}
	v, ok := parent.dirtyMap[int32(1)]
	if !ok {
		t.Fatalf("mvd.SetDirty() 后 parent.dirtyMap 应包含 key 1")
	}
	if !v {
		t.Fatalf("mvd.SetDirty() 后 parent.dirtyMap[key] 应为 true")
	}

	// parent 为 nil 时不 panic
	mvdNil := NewMapValueDirtyMark[int32](nil, 2)
	mvdNil.SetDirty() // 不应 panic

	// IsChanged() 永远返回 false
	if mvd.IsChanged() {
		t.Fatalf("MapValueDirtyMark.IsChanged 应永远返回 false")
	}

	// ResetChanged() 无操作
	mvd.ResetChanged() // 不应 panic, 无副作用
	if mvd.IsChanged() {
		t.Fatalf("ResetChanged 后 IsChanged 应仍为 false")
	}
}

// ==================== 辅助函数 Set / SetFn / MapSet / MapDel 测试 ====================

func TestSetFn(t *testing.T) {
	// Set() 修改字段值并 SetDirty
	d := &BaseDirtyMark{}
	var field int32 = 0
	Set(d, &field, 100)
	if field != 100 {
		t.Fatalf("Set 后 field 应为 100, 实际 %d", field)
	}
	if !d.IsChanged() {
		t.Fatalf("Set 后 IsChanged 应为 true")
	}
	if !d.IsDirty() {
		t.Fatalf("Set 后 IsDirty 应为 true")
	}

	// SetFn() 执行回调并 SetDirty
	d2 := &BaseDirtyMark{}
	var field2 int32 = 0
	SetFn(d2, func() {
		field2 = 200
	})
	if field2 != 200 {
		t.Fatalf("SetFn 后 field2 应为 200, 实际 %d", field2)
	}
	if !d2.IsChanged() {
		t.Fatalf("SetFn 后 IsChanged 应为 true")
	}
	if !d2.IsDirty() {
		t.Fatalf("SetFn 后 IsDirty 应为 true")
	}

	// MapSet() 添加元素并 SetDirty
	mark := &BaseMapDirtyMark{}
	m := map[int32]string{}
	MapSet(mark, m, 1, "hello")
	if m[1] != "hello" {
		t.Fatalf("MapSet 后 m[1] 应为 hello, 实际 %s", m[1])
	}
	if !mark.IsDirty() {
		t.Fatalf("MapSet 后 IsDirty 应为 true")
	}
	if !mark.IsChanged() {
		t.Fatalf("MapSet 后 IsChanged 应为 true")
	}

	// MapDel() 删除元素并 SetDirty
	MapDel(mark, m, 1)
	if _, ok := m[1]; ok {
		t.Fatalf("MapDel 后 m[1] 应被删除")
	}
	v, ok := mark.dirtyMap[int32(1)]
	if !ok {
		t.Fatalf("MapDel 后 dirtyMap 应包含 key 1")
	}
	if v {
		t.Fatalf("MapDel 后 dirtyMap[key] 应为 false")
	}
}
