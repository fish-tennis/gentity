package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gnet"
)

var (
	// 组件构造接口注册
	_playerComponentRegister = gentity.ComponentRegister[*testPlayer]{}
	// 消息回调接口注册
	_playerPacketHandlerRegister = gentity.NewMessageHandlerRegister()
	// 事件响应接口注册
	_playerEventHandlerRegister = gentity.NewEventHandlerRegister()
)

// 注册玩家组件构造信息
func registerPlayerComponentCtor(componentName string, ctorOrder int, ctor func(player *testPlayer, playerData *pb.PlayerData) gentity.Component) {
	_playerComponentRegister.Register(componentName, ctorOrder, func(entity *testPlayer, arg any) gentity.Component {
		return ctor(entity, arg.(*pb.PlayerData))
	})
}

func autoRegisterTestPlayer() {
	tmpPlayer := newTestPlayer(0, 0)
	// 消息回调接口注册
	_playerPacketHandlerRegister.AutoRegisterWithClient(tmpPlayer, gnet.NewDefaultConnectionHandler(nil),
		"On", "Handler", "example")
	// 事件响应接口注册
	_playerEventHandlerRegister.AutoRegister(tmpPlayer, "OnEvent")
}
