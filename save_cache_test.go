package gentity

import (
	"fmt"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/fish-tennis/gentity/examples/pb"
	"google.golang.org/protobuf/proto"
)

// ==================== Mock KvCache ====================

// mockKvCacheForSave 记录所有写入和删除操作,用于测试断言
// 注意: fix_verify_test.go 中已定义 mockKvCache,这里使用不同名称避免冲突
type mockKvCacheForSave struct {
	stringData  map[string]string            // Set 调用的 key -> value
	mapData     map[string]map[string]string // SetMap 调用的 key -> map[string]string(累积)
	delKeys     []string                     // Del 调用的 keys
	setCalls    int                          // Set 调用次数
	setMapCalls int                          // SetMap 调用次数
	hdelFields  []string                     // HDel 调用的 fields
}

func newMockKvCacheForSave() *mockKvCacheForSave {
	return &mockKvCacheForSave{
		stringData: make(map[string]string),
		mapData:    make(map[string]map[string]string),
	}
}

func (m *mockKvCacheForSave) Get(key string) (string, error) {
	return m.stringData[key], nil
}

func (m *mockKvCacheForSave) Set(key string, value interface{}, _ time.Duration) error {
	m.setCalls++
	switch v := value.(type) {
	case string:
		m.stringData[key] = v
	case proto.Message:
		bytes, _ := proto.Marshal(v)
		m.stringData[key] = string(bytes)
	default:
		m.stringData[key] = fmt.Sprintf("%v", v)
	}
	return nil
}

func (m *mockKvCacheForSave) SetNX(key string, value interface{}, _ time.Duration) (bool, error) {
	return true, nil
}

func (m *mockKvCacheForSave) Del(keys ...string) (int64, error) {
	m.delKeys = append(m.delKeys, keys...)
	for _, k := range keys {
		delete(m.stringData, k)
		delete(m.mapData, k)
	}
	return int64(len(keys)), nil
}

func (m *mockKvCacheForSave) Type(key string) (string, error) {
	if _, ok := m.stringData[key]; ok {
		return "string", nil
	}
	if _, ok := m.mapData[key]; ok {
		return "hash", nil
	}
	return "none", nil
}

func (m *mockKvCacheForSave) GetMap(key string, mapVal interface{}) error {
	return nil
}

func (m *mockKvCacheForSave) SetMap(key string, mapVal interface{}) error {
	m.setMapCalls++
	if m.mapData[key] == nil {
		m.mapData[key] = make(map[string]string)
	}
	val := reflect.ValueOf(mapVal)
	if val.Kind() == reflect.Map {
		iter := val.MapRange()
		for iter.Next() {
			k := fmt.Sprintf("%v", iter.Key().Interface())
			v := fmt.Sprintf("%v", iter.Value().Interface())
			m.mapData[key][k] = v
		}
	}
	return nil
}

func (m *mockKvCacheForSave) HGetAll(key string) (map[string]string, error) {
	return m.mapData[key], nil
}

func (m *mockKvCacheForSave) HSet(key string, values ...interface{}) (int64, error) {
	return 0, nil
}

func (m *mockKvCacheForSave) HSetNX(key, field string, value interface{}) (bool, error) {
	return true, nil
}

func (m *mockKvCacheForSave) HDel(key string, fields ...string) (int64, error) {
	m.hdelFields = append(m.hdelFields, fields...)
	if m.mapData[key] != nil {
		for _, f := range fields {
			delete(m.mapData[key], f)
		}
	}
	return int64(len(fields)), nil
}

func (m *mockKvCacheForSave) GetProto(key string, value proto.Message) error {
	return nil
}

// ==================== 测试结构 ====================

// testCacheProto 用于测试 BaseDirtyMark + proto 字段的缓存保存
type testCacheProto struct {
	BaseDirtyMark
	Data *pb.BaseInfo `db:""`
}

// ==================== Test: SaveChangedDataToCache (DirtyMark) ====================

func TestSaveChangedDataToCache_DirtyMark_Proto(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	obj := &testCacheProto{
		Data: &pb.BaseInfo{Level: 1},
	}
	obj.SetDirty()

	objStruct := GetObjSaveableStruct(obj)
	if objStruct == nil {
		t.Fatal("GetObjSaveableStruct returned nil")
	}
	fieldObj, field := objStruct.GetSingleSaveable(obj)
	if fieldObj == nil || field == nil {
		t.Fatal("GetSingleSaveable returned nil")
	}

	SaveChangedDataToCache(cache, fieldObj, "test-key", field)

	// 验证 cache.Set 被调用(proto 被序列化)
	if cache.setCalls == 0 {
		t.Error("cache.Set should be called for dirty proto data")
	}
	if _, ok := cache.stringData["test-key"]; !ok {
		t.Error("cache should have data for 'test-key'")
	}

	// 验证 dirtyMark 被重置
	if obj.IsDirty() {
		t.Error("dirtyMark should be reset after save")
	}
}

func TestSaveChangedDataToCache_DirtyMark_NotDirty(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	obj := &testCacheProto{
		Data: &pb.BaseInfo{Level: 1},
	}
	// 不调用 SetDirty

	objStruct := GetObjSaveableStruct(obj)
	fieldObj, field := objStruct.GetSingleSaveable(obj)

	SaveChangedDataToCache(cache, fieldObj, "test-key", field)

	// 验证 cache 没有写入
	if cache.setCalls > 0 {
		t.Errorf("cache.Set should NOT be called when not dirty, got %d calls", cache.setCalls)
	}
	if len(cache.stringData) > 0 {
		t.Errorf("cache should be empty, got: %v", cache.stringData)
	}
}

func TestSaveChangedDataToCache_MapDirtyMark(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	obj := NewMapData[int32, int32]()
	obj.Set(1, 10)
	obj.Set(2, 20)

	objStruct := GetObjSaveableStruct(obj)
	fieldObj, field := objStruct.GetSingleSaveable(obj)

	// 第一次保存: HasCached==false, 应全量写入
	SaveChangedDataToCache(cache, fieldObj, "map-key", field)

	if cache.setMapCalls != 1 {
		t.Errorf("first save: expected 1 SetMap call, got %d", cache.setMapCalls)
	}
	if cache.mapData["map-key"]["1"] != "10" {
		t.Errorf("first save: expected key 1 = 10, got %v", cache.mapData["map-key"]["1"])
	}
	if cache.mapData["map-key"]["2"] != "20" {
		t.Errorf("first save: expected key 2 = 20, got %v", cache.mapData["map-key"]["2"])
	}

	// 验证 HasCached 变为 true
	if !obj.HasCached() {
		t.Error("HasCached should be true after first save")
	}

	// 再次 Set 新数据(上一次保存已 ResetDirty)
	obj.Set(3, 30)

	// 第二次保存: HasCached==true, 应增量更新
	SaveChangedDataToCache(cache, fieldObj, "map-key", field)

	if cache.setMapCalls != 2 {
		t.Errorf("second save: expected 2 SetMap calls total, got %d", cache.setMapCalls)
	}
	// 增量更新: key 3 应被写入
	if cache.mapData["map-key"]["3"] != "30" {
		t.Errorf("incremental: expected key 3 = 30, got %v", cache.mapData["map-key"]["3"])
	}
}

// ==================== Test: SaveValueToCache ====================

func TestSaveValueToCache_Proto(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	val := reflect.ValueOf(&pb.BaseInfo{Level: 1})
	SaveValueToCache(cache, "key", val)

	if cache.setCalls == 0 {
		t.Error("cache.Set should be called for proto")
	}
	if _, ok := cache.stringData["key"]; !ok {
		t.Error("cache should have data for 'key'")
	}
}

func TestSaveValueToCache_Slice(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	val := reflect.ValueOf([]int32{1, 2, 3})
	SaveValueToCache(cache, "key", val)

	if cache.setCalls == 0 {
		t.Error("cache.Set should be called for slice")
	}
	// 验证值是 JSON 字符串
	v, ok := cache.stringData["key"]
	if !ok {
		t.Fatal("cache should have data for 'key'")
	}
	if v != "[1,2,3]" {
		t.Errorf("expected JSON '[1,2,3]', got %q", v)
	}
}

// ==================== Test: SaveMapValueToCache ====================

func TestSaveMapValueToCache_FirstTime(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	obj := NewMapData[int32, int32]()
	obj.Set(1, 10)
	obj.Set(2, 20)

	objStruct := GetObjSaveableStruct(obj)
	fieldObj, field := objStruct.GetSingleSaveable(obj)

	// 获取 Data 字段的 reflect.Value
	reflectVal := reflect.ValueOf(fieldObj)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}
	dataVal := reflectVal.Field(field.FieldIndex)

	SaveMapValueToCache(cache, "map-key", dataVal, obj)

	// HasCached==false 时应全量写入, SetMap 被调用
	if cache.setMapCalls != 1 {
		t.Errorf("expected 1 SetMap call for first time, got %d", cache.setMapCalls)
	}
	if cache.mapData["map-key"]["1"] != "10" {
		t.Errorf("expected key 1 = 10, got %v", cache.mapData["map-key"]["1"])
	}
	if cache.mapData["map-key"]["2"] != "20" {
		t.Errorf("expected key 2 = 20, got %v", cache.mapData["map-key"]["2"])
	}

	// HasCached 应变为 true
	if !obj.HasCached() {
		t.Error("HasCached should be true after first save")
	}
}

func TestSaveMapValueToCache_Incremental(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	cache := newMockKvCacheForSave()

	obj := NewMapData[int32, int32]()
	obj.Set(1, 10)
	obj.Set(2, 20)

	// 先标记为已缓存
	obj.SetCached()
	// 重置脏标记(模拟之前已全量保存过)
	obj.ResetDirty()

	// 设置增量脏标记: key=1 更新, key=2 删除
	obj.SetDirty(int32(1), true)
	obj.SetDirty(int32(2), false)

	objStruct := GetObjSaveableStruct(obj)
	fieldObj, field := objStruct.GetSingleSaveable(obj)

	reflectVal := reflect.ValueOf(fieldObj)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}
	dataVal := reflectVal.Field(field.FieldIndex)

	SaveMapValueToCache(cache, "map-key", dataVal, obj)

	// 增量更新: SetMap 应只包含 key=1
	if cache.setMapCalls != 1 {
		t.Errorf("expected 1 SetMap call for incremental, got %d", cache.setMapCalls)
	}
	if cache.mapData["map-key"]["1"] != "10" {
		t.Errorf("incremental: expected key 1 = 10, got %v", cache.mapData["map-key"]["1"])
	}
	if _, exists := cache.mapData["map-key"]["2"]; exists {
		t.Error("incremental: key 2 should not be in SetMap data")
	}

	// HDel 应被调用删除 key=2
	foundDel := false
	for _, f := range cache.hdelFields {
		if f == "2" {
			foundDel = true
		}
	}
	if !foundDel {
		t.Errorf("HDel should be called for key=2, got hdelFields: %v", cache.hdelFields)
	}
}
