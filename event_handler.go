package gentity

import (
	"reflect"
	"slices"
	"strings"
)

// 组件事件响应接口信息
type EventHandlerInfo struct {
	// 组件名,如果为空,就表示是直接写在Entity上的接口
	ComponentName string
	// 函数信息
	Method reflect.Method
	// 事件的reflect.Type
	EventType reflect.Type
}

// 组件事件响应接口管理类
type EventHandlerRegister struct {
	// 有事件回调的组件
	componentNames []string
	// 事件对应的回调接口列表,一对多
	eventHandlers map[reflect.Type][]*EventHandlerInfo // eventType -> handlers
}

func NewEventHandlerRegister() *EventHandlerRegister {
	return &EventHandlerRegister{
		eventHandlers:  make(map[reflect.Type][]*EventHandlerInfo),
		componentNames: make([]string, 0),
	}
}

func (this *EventHandlerRegister) AddEventHandlerInfo(eventHandlerInfo *EventHandlerInfo) {
	this.eventHandlers[eventHandlerInfo.EventType] = append(this.eventHandlers[eventHandlerInfo.EventType], eventHandlerInfo)
	if !slices.Contains(this.componentNames, eventHandlerInfo.ComponentName) {
		this.componentNames = append(this.componentNames, eventHandlerInfo.ComponentName)
	}
}

// 扫描一个struct的函数
func (this *EventHandlerRegister) scanMethods(obj any, methodNamePrefix string) {
	typ := reflect.TypeOf(obj)
	componentName := ""
	component, ok := obj.(Component)
	if ok {
		componentName = component.GetName()
	}
	// 如: game.Quest -> Quest
	componentStructName := typ.String()[strings.LastIndex(typ.String(), ".")+1:]
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		if method.Type.NumIn() != 2 {
			continue
		}
		// 函数名前缀检查
		if !strings.HasPrefix(method.Name, methodNamePrefix) {
			continue
		}
		// 事件回调格式: func (this *Quest) OnEventPlayerEntryGame(evt *PlayerEntryGame)
		eventType := method.Type.In(1)
		eventHandlerInfo := &EventHandlerInfo{
			ComponentName: componentName,
			Method:        method,
			EventType:     eventType,
		}
		this.AddEventHandlerInfo(eventHandlerInfo)
		GetLogger().Info("EventHandler %v.%v event:%v", componentStructName, method.Name, eventType.String())
	}
}

// 扫描entity以及entity的组件,寻找匹配格式的事件响应接口
func (this *EventHandlerRegister) AutoRegister(entity Entity, methodNamePrefix string) {
	// entity上的回调
	this.scanMethods(entity, methodNamePrefix)
	// 组件上的事件回调
	entity.RangeComponent(func(component Component) bool {
		this.scanMethods(component, methodNamePrefix)
		return true
	})
}

// 响应事件
//
//	如果没有注册对应事件的响应接口,return false
func (this *EventHandlerRegister) Invoke(entity Entity, evt any) bool {
	eventType := reflect.TypeOf(evt)
	handlers := this.eventHandlers[eventType]
	if len(handlers) == 0 {
		return false
	}
	for _, handler := range handlers {
		if handler.ComponentName != "" {
			// 组件上的事件响应接口
			component := entity.GetComponentByName(handler.ComponentName)
			handler.Method.Func.Call([]reflect.Value{reflect.ValueOf(component),
				reflect.ValueOf(evt)})
		} else {
			// Entitys上的事件响应接口
			handler.Method.Func.Call([]reflect.Value{reflect.ValueOf(entity),
				reflect.ValueOf(evt)})
		}
	}
	return true
}
