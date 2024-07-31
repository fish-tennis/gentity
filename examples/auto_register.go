package examples

import (
	"github.com/fish-tennis/gentity"
)

var (
	// 组件构造接口注册
	_playerComponentRegister = gentity.ComponentRegister[*Player]{}
	// 事件响应接口注册
	_playerEventHandlerMgr = gentity.NewEventHandlerMgr()
)

func autoRegisterTestPlayer() {
	tmpPlayer := newTestPlayer(0, 0)
	// 事件响应接口注册
	_playerEventHandlerMgr.AutoRegister(tmpPlayer, "Trigger")
}
