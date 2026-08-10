package gentity

import (
	"log/slog"
	"testing"
)

// ==================== 测试辅助类型 ====================

// testComp 嵌入 BaseComponent,自动满足 Component 接口
type testComp struct {
	BaseComponent
}

func newTestComp(name string) *testComp {
	return &testComp{BaseComponent: BaseComponent{name: name}}
}

// testEventReceiverComp 实现 EventReceiver 接口
type testEventReceiverComp struct {
	BaseComponent
	received []interface{}
}

func (c *testEventReceiverComp) OnEvent(event interface{}) {
	c.received = append(c.received, event)
}

// ==================== BaseEntity 测试 ====================

// TestBaseEntity_AddComponent 测试组件的添加、按名查找、遍历和按索引获取
func TestBaseEntity_AddComponent(t *testing.T) {
	e := &BaseEntity{}
	c1 := newTestComp("Comp1")
	c2 := newTestComp("Comp2")
	e.AddComponent(c1)
	e.AddComponent(c2)

	// GetComponentByName 分别能找到
	if e.GetComponentByName("Comp1") != c1 {
		t.Fatal("GetComponentByName Comp1 failed")
	}
	if e.GetComponentByName("Comp2") != c2 {
		t.Fatal("GetComponentByName Comp2 failed")
	}
	// 不存在的组件返回 nil
	if e.GetComponentByName("NotExist") != nil {
		t.Fatal("GetComponentByName should return nil for non-exist component")
	}

	// RangeComponent 遍历所有组件
	count := 0
	e.RangeComponent(func(component Component) bool {
		count++
		return true
	})
	if count != 2 {
		t.Fatalf("expected 2 components, got %d", count)
	}

	// GetComponentByIndex 按索引获取
	if e.GetComponentByIndex(0) != c1 {
		t.Fatal("GetComponentByIndex(0) failed")
	}
	if e.GetComponentByIndex(1) != c2 {
		t.Fatal("GetComponentByIndex(1) failed")
	}
}

// TestBaseEntity_DuplicateComponent 测试添加同名组件时第二次被拒绝
func TestBaseEntity_DuplicateComponent(t *testing.T) {
	// 抑制预期的错误日志输出
	oldLevel := slog.SetLogLoggerLevel(slog.LevelError + 1)
	defer func() { slog.SetLogLoggerLevel(oldLevel) }()

	e := &BaseEntity{}
	c1 := newTestComp("Dup")
	c2 := newTestComp("Dup")
	e.AddComponent(c1)
	// 第二次 AddComponent 同名,应被拒绝(不报错但不添加)
	e.AddComponent(c2)

	// GetComponentByName 返回第一次的组件
	if e.GetComponentByName("Dup") != c1 {
		t.Fatal("GetComponentByName should return the first component")
	}
	// 组件数量仍为1
	if len(e.GetComponents()) != 1 {
		t.Fatalf("expected 1 component, got %d", len(e.GetComponents()))
	}
}

// TestBaseComponent 测试 BaseComponent 的基础行为
func TestBaseComponent(t *testing.T) {
	bc := NewBaseComponent(nil, "Test")
	if bc.GetName() != "Test" {
		t.Fatalf("expected name 'Test', got %q", bc.GetName())
	}
	if bc.GetEntity() != nil {
		t.Fatal("expected nil entity")
	}
	// SetEntity 后 GetEntity 返回新值
	e := &BaseEntity{}
	bc.SetEntity(e)
	if bc.GetEntity() != e {
		t.Fatal("GetEntity should return the entity set by SetEntity")
	}
}

// ==================== ComponentRegister 测试 ====================

// TestComponentRegister_Order 测试按 CtorOrder 顺序构造组件
func TestComponentRegister_Order(t *testing.T) {
	cr := &ComponentRegister[*BaseEntity]{}
	cr.Register("Comp30", 30, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Comp30")
	})
	cr.Register("Comp10", 10, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Comp10")
	})
	cr.Register("Comp20", 20, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Comp20")
	})

	e := &BaseEntity{}
	cr.InitComponents(e, nil)

	// 验证组件添加顺序为 CtorOrder=10, 20, 30
	if e.GetComponentByIndex(0).GetName() != "Comp10" {
		t.Fatalf("index 0 expected Comp10, got %v", e.GetComponentByIndex(0).GetName())
	}
	if e.GetComponentByIndex(1).GetName() != "Comp20" {
		t.Fatalf("index 1 expected Comp20, got %v", e.GetComponentByIndex(1).GetName())
	}
	if e.GetComponentByIndex(2).GetName() != "Comp30" {
		t.Fatalf("index 2 expected Comp30, got %v", e.GetComponentByIndex(2).GetName())
	}
	if len(e.GetComponents()) != 3 {
		t.Fatalf("expected 3 components, got %d", len(e.GetComponents()))
	}
}

// TestComponentRegister_Duplicate 测试注册同名组件时第二次被忽略
func TestComponentRegister_Duplicate(t *testing.T) {
	cr := &ComponentRegister[*BaseEntity]{}
	cr.Register("Dup", 10, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Dup")
	})
	// 同名第二次注册应被忽略
	cr.Register("Dup", 20, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Dup-other")
	})
	if len(cr.RegisterInfos) != 1 {
		t.Fatalf("expected 1 register info, got %d", len(cr.RegisterInfos))
	}
}

// TestComponentRegister_NilComponent 测试构造函数返回 nil 时不 panic 且不添加
func TestComponentRegister_NilComponent(t *testing.T) {
	cr := &ComponentRegister[*BaseEntity]{}
	cr.Register("Nil", 10, func(entity *BaseEntity, arg any) Component {
		return nil
	})
	cr.Register("Real", 20, func(entity *BaseEntity, arg any) Component {
		return newTestComp("Real")
	})

	e := &BaseEntity{}
	// 构造函数返回 nil,InitComponents 不应 panic
	cr.InitComponents(e, nil)

	// nil 组件不会被添加
	if e.GetComponentByName("Nil") != nil {
		t.Fatal("nil component should not be added")
	}
	// 正常组件被添加
	if e.GetComponentByName("Real") == nil {
		t.Fatal("real component should be added")
	}
	if len(e.GetComponents()) != 1 {
		t.Fatalf("expected 1 component, got %d", len(e.GetComponents()))
	}
}

// TestBaseEntity_EventReceiver 测试事件接收器的自动注册与手动管理
func TestBaseEntity_EventReceiver(t *testing.T) {
	e := &BaseEntity{}
	c1 := &testEventReceiverComp{BaseComponent: BaseComponent{name: "ER1"}}

	// AddComponent 后,实现 EventReceiver 的组件自动加入 eventReceivers
	e.AddComponent(c1)

	// 验证 c1 已自动加入 eventReceivers
	count := 0
	foundER1 := false
	e.RangeEventReceiver(func(er EventReceiver) bool {
		count++
		if er == c1 {
			foundER1 = true
		}
		return true
	})
	if count != 1 {
		t.Fatalf("expected 1 event receiver, got %d", count)
	}
	if !foundER1 {
		t.Fatal("component c1 not auto-added to eventReceivers")
	}

	// AddEventReceiver 添加另一个 EventReceiver
	c2 := &testEventReceiverComp{BaseComponent: BaseComponent{name: "ER2"}}
	e.AddEventReceiver(c2)

	count = 0
	foundER2 := false
	e.RangeEventReceiver(func(er EventReceiver) bool {
		count++
		if er == c2 {
			foundER2 = true
		}
		return true
	})
	if count != 2 {
		t.Fatalf("expected 2 event receivers, got %d", count)
	}
	if !foundER2 {
		t.Fatal("component c2 not in eventReceivers")
	}

	// AddEventReceiver 重复添加同一个不会重复
	e.AddEventReceiver(c2)
	count = 0
	e.RangeEventReceiver(func(er EventReceiver) bool {
		count++
		return true
	})
	if count != 2 {
		t.Fatalf("expected 2 event receivers after duplicate add, got %d", count)
	}

	// 验证事件分发可用
	c1.OnEvent("evt1")
	c2.OnEvent("evt2")
	if len(c1.received) != 1 || c1.received[0] != "evt1" {
		t.Fatalf("c1 event receive unexpected: %v", c1.received)
	}
	if len(c2.received) != 1 || c2.received[0] != "evt2" {
		t.Fatalf("c2 event receive unexpected: %v", c2.received)
	}
}
