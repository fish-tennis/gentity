package gentity

import (
	"reflect"
	"testing"
	"time"
)

// ==================== LoadObjData 多child continue 修复验证 ====================

// testMultiChildComponent 用于测试多child组件的LoadObjData
// 模拟背包有多个子模块,第一个child加载失败时,后续child仍应正常加载
type testMultiChildComponent struct {
	*BaseComponent
	Child1 *MapData[int32, int32] `child:"Child1"`
	Child2 *MapData[int32, int32] `child:"Child2"`
	Child3 *MapData[int32, int32] `child:"Child3"`
}

// TestLoadObjData_MultiChild_ContinueWhenError 验证多child加载时,某个child出错不会跳过后续child
func TestLoadObjData_MultiChild_ContinueWhenError(t *testing.T) {
	SetLogLevel(-1) // 静默错误日志

	// 构造一个多child组件,其中Child1的数据故意设为错误类型
	comp := &testMultiChildComponent{
		BaseComponent: NewBaseComponent(nil, "TestMultiChild"),
		Child1:        NewMapData[int32, int32](),
		Child2:        NewMapData[int32, int32](),
		Child3:        NewMapData[int32, int32](),
	}

	// sourceData: Child1的数据故意设为错误类型,Child2/Child3正常
	sourceData := map[string]any{
		"Child1": "invalid-data", // 故意传错误类型,导致loadField失败
		"Child2": map[int32]int32{1: 20},
		"Child3": map[int32]int32{2: 30},
	}

	// LoadObjData 应返回nil(不因某个child失败而return错误)
	err := LoadObjData(comp, sourceData)
	if err != nil {
		t.Fatalf("LoadObjData should return nil when a child fails, got: %v", err)
	}

	// Child2 应被成功加载
	if v, ok := comp.Child2.Get(1); !ok || v != 20 {
		t.Errorf("Child2 should be loaded: got v=%v ok=%v", v, ok)
	}
	// Child3 应被成功加载
	if v, ok := comp.Child3.Get(2); !ok || v != 30 {
		t.Errorf("Child3 should be loaded: got v=%v ok=%v", v, ok)
	}
}

// TestLoadObjData_MultiChild_MissingField 验证缺少某个child数据时,后续child仍正常
func TestLoadObjData_MultiChild_MissingField(t *testing.T) {
	SetLogLevel(-1)

	comp := &testMultiChildComponent{
		BaseComponent: NewBaseComponent(nil, "TestMultiChild2"),
		Child1:        NewMapData[int32, int32](),
		Child2:        NewMapData[int32, int32](),
		Child3:        NewMapData[int32, int32](),
	}

	// sourceData 中缺少 Child1
	sourceData := map[string]any{
		"Child2": map[int32]int32{10: 100},
		"Child3": map[int32]int32{20: 200},
	}

	err := LoadObjData(comp, sourceData)
	if err != nil {
		t.Fatalf("LoadObjData should return nil: %v", err)
	}

	if v, ok := comp.Child2.Get(10); !ok || v != 100 {
		t.Errorf("Child2 should be loaded: got v=%v ok=%v", v, ok)
	}
	if v, ok := comp.Child3.Get(20); !ok || v != 200 {
		t.Errorf("Child3 should be loaded: got v=%v ok=%v", v, ok)
	}
}

// ==================== LoadObjData InterfaceMap 传参修正验证 ====================

// testInterfaceMapChildComponent 测试InterfaceMap child传参
type testInterfaceMapChildComponent struct {
	*BaseComponent
	NormalChild  *MapData[int32, int32] `child:"NormalChild"`
	IfaceMapData map[int32]any          `child:"IfaceMap"`
}

// 实现InterfaceMapLoader,记录收到的数据
type testInterfaceMapLoader struct {
	receivedData any
}

func (l *testInterfaceMapLoader) LoadFromBytesMap(bytesMap any) error {
	l.receivedData = bytesMap
	return nil
}

// TestLoadObjData_InterfaceMapChild_CorrectParam 验证InterfaceMap child收到的是自身数据而非整个sourceData
func TestLoadObjData_InterfaceMapChild_CorrectParam(t *testing.T) {
	SetLogLevel(-1)

	loader := &testInterfaceMapLoader{}

	// 验证 LoadFromBytesMap 收到的是 child 对应的 sourceFieldVal 而非整个 sourceData
	childData := map[string][]byte{"1": []byte("item1")}
	otherData := map[string]int32{"2": 99} // NormalChild的数据

	sourceData := map[string]any{
		"NormalChild": otherData,
		"IfaceMap":    childData, // InterfaceMap child自己的数据
	}

	// 模拟LoadObjData中的InterfaceMap处理逻辑
	sourceVal := reflect.ValueOf(sourceData)
	sourceFieldVal := GetFieldValue(sourceVal, "IfaceMap")
	loader.LoadFromBytesMap(sourceFieldVal.Interface())

	// loader收到的应该是childData(IfaceMap自身的数据),而不是整个sourceData
	received, ok := loader.receivedData.(map[string][]byte)
	if !ok {
		t.Fatalf("receivedData should be map[string][]byte, got: %T", loader.receivedData)
	}
	// 验证收到的是childData(有key="1"),而不是otherData(key="2")
	if _, hasKey := received["1"]; !hasKey {
		t.Errorf("InterfaceMap child should receive its own data, not the whole sourceData")
	}
	if _, hasWrongKey := received["2"]; hasWrongKey {
		t.Errorf("InterfaceMap child should not receive other child's data")
	}
}

// ==================== TimerEntries Run+AddTimer 不跳过验证 ====================

// TestTimerEntries_RunAddTimerNoSkip 验证Run期间AddTimer不会导致同批到期timer被跳过
func TestTimerEntries_RunAddTimerNoSkip(t *testing.T) {
	SetLogLevel(-1)
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()

	var callOrder []int

	// timer1: 到期, job中AddTimer(timer3)
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		callOrder = append(callOrder, 1)
		// 在job中添加新的timer(timer3),也设为已到期
		te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
			callOrder = append(callOrder, 3)
			return 0 // 不重复
		})
		return 0 // timer1不重复
	})

	// timer2: 同样到期,应在timer1之后被执行,不应被sort跳过
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		callOrder = append(callOrder, 2)
		return 0 // 不重复
	})

	// Run执行
	te.Run(time.Now())

	// timer1和timer2都应被执行,顺序应为1,2
	// timer3是Run期间新加的,本轮不应执行(下次Run才执行)
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != 1 || callOrder[1] != 2 {
		t.Errorf("expected call order [1,2], got %v", callOrder)
	}

	// 第二次Run: timer3应该被执行
	callOrder = nil
	te.Run(time.Now())

	if len(callOrder) != 1 || callOrder[0] != 3 {
		t.Errorf("timer3 should execute in second Run, got: %v", callOrder)
	}
}

// TestTimerEntries_RunAddTimerWithRecurring 验证recurring timer + AddTimer场景
func TestTimerEntries_RunAddTimerWithRecurring(t *testing.T) {
	SetLogLevel(-1)
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()

	var calls []int

	// timer A: recurring, job中AddTimer(timer C)
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		calls = append(calls, 1)
		// 添加timer C,设为已到期
		te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
			calls = append(calls, 3)
			return 0
		})
		return 10 * time.Minute // A返回很长的间隔,不会立即重复
	})

	// timer B: 到期
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		calls = append(calls, 2)
		return 0
	})

	te.Run(time.Now())

	// A和B都应执行,顺序1,2; C是新加的,本轮不执行
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(calls), calls)
	}
	if calls[0] != 1 || calls[1] != 2 {
		t.Errorf("expected [1,2], got %v", calls)
	}
}

// ==================== convertValueToStringOrInterface nil 处理验证 ====================

// TestConvertValueToStringOrInterface_NilPtr 验证nil指针返回明确错误
func TestConvertValueToStringOrInterface_NilPtr(t *testing.T) {
	SetLogLevel(-1)

	// nil指针
	var nilPtr *int
	val := reflect.ValueOf(nilPtr)
	result, err := convertValueToStringOrInterface(val)
	if err == nil {
		t.Fatal("should return error for nil ptr")
	}
	if result != nil {
		t.Errorf("should return nil result, got: %v", result)
	}
	if err.Error() != "value is nil" {
		t.Errorf("error message should be 'value is nil', got: %q", err.Error())
	}
}

// TestConvertValueToStringOrInterface_NilInterface 验证nil interface返回明确错误
func TestConvertValueToStringOrInterface_NilInterface(t *testing.T) {
	SetLogLevel(-1)

	// nil interface比较特殊,reflect.ValueOf(nil)得到invalid Value
	// 测试包含nil值的interface
	var nilIface interface{} = (*int)(nil)
	val := reflect.ValueOf(nilIface)
	if val.Kind() != reflect.Ptr {
		t.Fatalf("expected Ptr kind, got %v", val.Kind())
	}
	result, err := convertValueToStringOrInterface(val)
	if err == nil {
		t.Fatal("should return error for nil interface")
	}
	if result != nil {
		t.Errorf("should return nil result, got: %v", result)
	}
}

// TestConvertValueToStringOrInterface_NonNilPtr 验证非nil指针正常工作(回归测试)
func TestConvertValueToStringOrInterface_NonNilPtr(t *testing.T) {
	SetLogLevel(-1)

	// 非nil的int指针
	v := 42
	val := reflect.ValueOf(&v)
	result, err := convertValueToStringOrInterface(val)
	if err != nil {
		t.Fatalf("should not return error for non-nil ptr: %v", err)
	}
	// int*不在proto.Message或Saveable的分支,应返回interface{}值
	if result == nil {
		t.Error("should return non-nil result")
	}
}

// TestConvertValueToStringOrInterface_Bool 回归测试bool仍然正常
func TestConvertValueToStringOrInterface_Bool(t *testing.T) {
	SetLogLevel(-1)

	val := reflect.ValueOf(true)
	result, err := convertValueToStringOrInterface(val)
	if err != nil {
		t.Fatalf("bool should not error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if s != "true" {
		t.Errorf("expected 'true', got %q", s)
	}
}

// ==================== FixEntityDataFromCache 多child continue 回归验证 ====================

// mockEntityDb 用于测试FixEntityDataFromCache
type mockEntityDb struct {
	savedFields map[string]interface{}
}

func (m *mockEntityDb) FindEntityById(interface{}, interface{}) (bool, error) { return false, nil }
func (m *mockEntityDb) InsertEntity(interface{}, interface{}) (error, bool)   { return nil, false }
func (m *mockEntityDb) SaveEntity(interface{}, interface{}) error             { return nil }
func (m *mockEntityDb) DeleteEntity(interface{}) error                       { return nil }
func (m *mockEntityDb) SaveComponent(interface{}, string, interface{}) error { return nil }
func (m *mockEntityDb) SaveComponents(interface{}, map[string]interface{}) error {
	return nil
}
func (m *mockEntityDb) SaveComponentField(entityKey interface{}, fieldName string, fieldData interface{}) error {
	if m.savedFields == nil {
		m.savedFields = make(map[string]interface{})
	}
	m.savedFields[fieldName] = fieldData
	return nil
}
func (m *mockEntityDb) DeleteComponentField(interface{}, string, ...string) error { return nil }

// mockKvCache 用于测试FixEntityDataFromCache
type mockKvCache struct {
	data map[string]string // key -> value(string类型)
}

func (m *mockKvCache) Get(key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", nil // 模拟key不存在
	}
	return v, nil
}
func (m *mockKvCache) Set(key string, value interface{}, _ time.Duration) error { return nil }
func (m *mockKvCache) SetNX(key string, value interface{}, _ time.Duration) (bool, error) {
	return true, nil
}
func (m *mockKvCache) Del(keys ...string) (int64, error) {
	for _, k := range keys {
		delete(m.data, k)
	}
	return int64(len(keys)), nil
}
func (m *mockKvCache) Type(key string) (string, error) {
	if _, ok := m.data[key]; ok {
		return "string", nil
	}
	return "none", nil
}
func (m *mockKvCache) GetMap(key string, mapVal interface{}) error { return nil }
func (m *mockKvCache) SetMap(key string, m2 interface{}) error     { return nil }
func (m *mockKvCache) HGetAll(key string) (map[string]string, error) {
	return nil, nil
}
func (m *mockKvCache) HSet(key string, values ...interface{}) (int64, error) {
	return 0, nil
}
func (m *mockKvCache) HSetNX(key, field string, value interface{}) (bool, error) {
	return true, nil
}
func (m *mockKvCache) HDel(key string, fields ...string) (int64, error) {
	return 0, nil
}
func (m *mockKvCache) GetProto(key string, value interface{}) error { return nil }
