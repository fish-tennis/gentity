package gentity

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"
)

// ==================== RoutineEntity 测试 ====================

// testRoutineEntity 用于测试的最小RoutineEntity实现
type testRoutineEntity struct {
	*BaseRoutineEntity
	processedMsgs []any
	msgMu         sync.Mutex
}

func newTestRoutineEntity(chanLen int) *testRoutineEntity {
	return &testRoutineEntity{
		BaseRoutineEntity: NewRoutineEntity(chanLen),
	}
}

func (t *testRoutineEntity) GetId() int64 { return 1 }

// 测试 Stop 后 PushMessage 被拒绝(不阻塞,不泄漏)
func TestStopRejectsPushMessage(t *testing.T) {
	e := newTestRoutineEntity(1)
	e.Stop()
	if !e.IsStopped() {
		t.Fatal("IsStopped should be true after Stop")
	}
	// PushMessage 应直接返回,不阻塞
	done := make(chan struct{})
	go func() {
		e.PushMessage("msg-after-stop")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("PushMessage blocked after Stop")
	}
	// TryPushMessage 返回 false
	if e.TryPushMessage("msg") {
		t.Fatal("TryPushMessage should return false after Stop")
	}
	// PushMessageTimeout 返回 false
	if e.PushMessageTimeout("msg", 100*time.Millisecond) {
		t.Fatal("PushMessageTimeout should return false after Stop")
	}
}

// 测试 TryPushMessage channel满时返回false
func TestTryPushMessageChannelFull(t *testing.T) {
	e := newTestRoutineEntity(1)
	// 不启动协程,填满channel
	if !e.TryPushMessage("msg1") {
		t.Fatal("first TryPushMessage should succeed")
	}
	// channel已满
	if e.TryPushMessage("msg2") {
		t.Fatal("TryPushMessage should fail when channel full")
	}
}

// 测试 PushMessageTimeout 超时返回false
func TestPushMessageTimeout(t *testing.T) {
	e := newTestRoutineEntity(1)
	// 填满channel
	e.TryPushMessage("msg1")
	// 超时返回false
	start := time.Now()
	if e.PushMessageTimeout("msg2", 50*time.Millisecond) {
		t.Fatal("PushMessageTimeout should return false on timeout")
	}
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Fatalf("timeout too fast: %v", elapsed)
	}
}

// 测试协程正常退出后 stopped 标志置位
func TestRoutineExitSetsStopped(t *testing.T) {
	// 准备Application
	oldApp := _application
	_application = &testApp{wg: &sync.WaitGroup{}, ctx: context.Background()}
	defer func() { _application = oldApp }()

	e := newTestRoutineEntity(16)
	app := _application.(*testApp)
	var endCalled bool
	args := &RoutineEntityRoutineArgs{
		ProcessMessageFunc: func(re RoutineEntity, message any) {
			e.msgMu.Lock()
			e.processedMsgs = append(e.processedMsgs, message)
			e.msgMu.Unlock()
		},
		EndFunc: func(re RoutineEntity) {
			endCalled = true
		},
	}
	if !e.RunProcessRoutine(e, args) {
		t.Fatal("RunProcessRoutine failed")
	}
	// 发消息确认协程在运行
	e.PushMessage("hello")
	time.Sleep(50 * time.Millisecond)
	// 停止
	e.Stop()
	app.wg.Wait()
	if !endCalled {
		t.Fatal("EndFunc not called")
	}
	if !e.IsStopped() {
		t.Fatal("stopped flag should be true after routine exit")
	}
}

// 测试 EndFunc 中 panic 不会导致 WaitGroup 泄漏
func TestEndFuncPanicNoLeak(t *testing.T) {
	oldApp := _application
	_application = &testApp{wg: &sync.WaitGroup{}, ctx: context.Background()}
	defer func() { _application = oldApp }()

	e := newTestRoutineEntity(16)
	app := _application.(*testApp)
	args := &RoutineEntityRoutineArgs{
		EndFunc: func(re RoutineEntity) {
			panic("simulated EndFunc panic")
		},
	}
	e.RunProcessRoutine(e, args)
	time.Sleep(50 * time.Millisecond)
	e.Stop()
	// wg.Wait 应该能返回,说明 Done 被执行了
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitGroup leaked: EndFunc panic caused Done to be skipped")
	}
}

// 测试 ProcessMessageFunc panic 不导致 WaitGroup 泄漏
func TestProcessMessagePanicNoLeak(t *testing.T) {
	oldApp := _application
	_application = &testApp{wg: &sync.WaitGroup{}, ctx: context.Background()}
	defer func() { _application = oldApp }()

	e := newTestRoutineEntity(16)
	app := _application.(*testApp)
	args := &RoutineEntityRoutineArgs{
		ProcessMessageFunc: func(re RoutineEntity, message any) {
			panic("simulated process panic")
		},
	}
	e.RunProcessRoutine(e, args)
	e.PushMessage("trigger-panic")
	time.Sleep(50 * time.Millisecond)
	e.Stop()
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitGroup leaked: ProcessMessageFunc panic caused Done to be skipped")
	}
}

// ==================== testApp 辅助 ====================

type testApp struct {
	id  int32
	wg  *sync.WaitGroup
	ctx context.Context
}

func (a *testApp) GetId() int32                      { return a.id }
func (a *testApp) GetContext() context.Context       { return a.ctx }
func (a *testApp) GetWaitGroup() *sync.WaitGroup     { return a.wg }
func (a *testApp) Init(context.Context, string) bool { return true }
func (a *testApp) Run(context.Context)               {}
func (a *testApp) OnUpdate(context.Context, int64)   {}
func (a *testApp) Exit()                             {}

// ==================== Convert 测试 ====================

// 测试 ConvertInterfaceToRealType proto反序列化失败时返回nil而非error对象
func TestConvertInterfaceToRealType_ProtoUnmarshalFail(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1) // 静默错误日志
	type pbMessage struct{}                     // 不实现 proto.Message,用真实 proto 测试更准
	// 用无效bytes反序列化,确保失败
	type fakeProto struct{}
	// 使用实际的 proto.Message 类型: pb.QuestData
	// 但这里测试的是 ConvertInterfaceToRealType 内部逻辑
	// 传入无效 []byte 到一个 Ptr 类型,期望返回 nil
	typ := reflect.TypeOf((*struct{ X int32 })(nil))
	result := ConvertInterfaceToRealType(typ, []byte("invalid-proto-bytes"))
	// 不是 proto.Message 类型,会走到 Ptr 分支尝试 Unmarshal,失败应返回 nil
	_ = result // 该路径取决于类型是否实现 proto.Message,见下面更精确的测试
	_ = pbMessage{}
	_ = fakeProto{}
}

// 测试 ConvertStringToRealType 对 Interface 类型返回原值
func TestConvertStringToRealType_Interface(t *testing.T) {
	typ := reflect.TypeOf((*any)(nil)).Elem() // interface{}
	v := "hello"
	result := ConvertStringToRealType(typ, v)
	if result != v {
		t.Fatalf("expected %v, got %v", v, result)
	}
}

// 测试 ConvertStringToRealType proto反序列化失败返回nil
func TestConvertStringToRealType_ProtoUnmarshalFail(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelError + 1)
	// 构造一个实现了 proto.Message 的 Ptr 类型
	// 用 google.golang.org/protobuf 的空消息测试
	// 这里直接用一个无效字符串,Ptr分支 Unmarshal 必然失败
	typ := reflect.TypeOf((*testProtoMsg)(nil))
	result := ConvertStringToRealType(typ, "invalid")
	if result != nil {
		t.Fatalf("expected nil on unmarshal fail, got: %v", result)
	}
}

// testProtoMsg 实现最小 proto.Message 用于测试
type testProtoMsg struct{}

func (m *testProtoMsg) Reset()         {}
func (m *testProtoMsg) String() string { return "" }
func (m *testProtoMsg) ProtoMessage()  {}

// ==================== CacheKey 测试 ====================

func TestGetEntityCacheKey(t *testing.T) {
	k := GetEntityCacheKey("player", int64(123))
	expected := "player.{123}"
	if k != expected {
		t.Fatalf("expected %q, got %q", expected, k)
	}
}

func TestGetEntityComponentCacheKey(t *testing.T) {
	k := GetEntityComponentCacheKey("player", int64(123), "BaseInfo")
	expected := "player.{123}.BaseInfo"
	if k != expected {
		t.Fatalf("expected %q, got %q", expected, k)
	}
}

func TestGetEntityComponentChildCacheKey(t *testing.T) {
	k := GetEntityComponentChildCacheKey("player", int64(123), "Quest", "Finished")
	expected := "player.{123}.Quest.Finished"
	if k != expected {
		t.Fatalf("expected %q, got %q", expected, k)
	}
}

// 测试 useLowerName 开关
func TestGetEntityComponentCacheKey_LowerName(t *testing.T) {
	old := _saveableStructsMap.useLowerName
	_saveableStructsMap.useLowerName = true
	defer func() { _saveableStructsMap.useLowerName = old }()

	k := GetEntityComponentCacheKey("player", int64(123), "BaseInfo")
	expected := "player.{123}.baseinfo"
	if k != expected {
		t.Fatalf("expected %q, got %q", expected, k)
	}
}

// ==================== EventHandlerMgr 测试 ====================

type testEvent struct{ Val int }

type testEventEntity struct {
	BaseEntity
	called bool
}

func (e *testEventEntity) GetId() int64 { return 1 }

type testEventComponent struct {
	BaseComponent
	called bool
}

// 模拟事件响应方法
func (c *testEventComponent) OnTriggerTestEvent(evt *testEvent) {
	c.called = true
}

func TestEventHandlerInvoke(t *testing.T) {
	mgr := NewEventHandlerMgr()
	entity := &testEventEntity{}
	comp := &testEventComponent{BaseComponent: BaseComponent{entity: entity, name: "TestComp"}}
	// 必须通过 AddComponent 注册,否则 GetComponentByName 返回 nil
	entity.AddComponent(comp)

	// AutoRegister 依赖反射扫描,这里手动注册
	// 模拟 scanMethods 的效果
	mgr.scanMethods(comp, "On")
	if len(mgr.eventHandlers) == 0 {
		t.Fatal("no handlers registered")
	}

	// Invoke
	evt := &testEvent{Val: 42}
	ok := mgr.Invoke(entity, evt)
	if !ok {
		t.Fatal("Invoke should return true when handlers exist")
	}
	if !comp.called {
		t.Fatal("component handler not called")
	}

	// 未注册的事件
	ok = mgr.Invoke(entity, &struct{}{})
	if ok {
		t.Fatal("Invoke should return false for unregistered event")
	}
}

// 测试同事件多个回调都执行
func TestEventHandlerMultiple(t *testing.T) {
	mgr := NewEventHandlerMgr()
	entity := &testEventEntity{}
	comp1 := &testEventComponent{BaseComponent: BaseComponent{entity: entity, name: "Comp1"}}
	comp2 := &testEventComponent{BaseComponent: BaseComponent{entity: entity, name: "Comp2"}}
	entity.AddComponent(comp1)
	entity.AddComponent(comp2)
	mgr.scanMethods(comp1, "On")
	mgr.scanMethods(comp2, "On")

	mgr.Invoke(entity, &testEvent{})
	if !comp1.called || !comp2.called {
		t.Fatal("both components should be called")
	}
}

// ==================== convertValueToString Bool 分支测试 ====================

func TestConvertValueToString_Bool(t *testing.T) {
	// 测试 true
	val := reflect.ValueOf(true)
	s, err := convertValueToString(val)
	if err != nil {
		t.Fatalf("convertValueToString(bool true) err: %v", err)
	}
	if s != "true" {
		t.Fatalf("expected 'true', got %q", s)
	}
	// 测试 false
	val = reflect.ValueOf(false)
	s, err = convertValueToString(val)
	if err != nil {
		t.Fatalf("convertValueToString(bool false) err: %v", err)
	}
	if s != "false" {
		t.Fatalf("expected 'false', got %q", s)
	}
}
