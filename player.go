package gentity

import (
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

type Player interface {
	Entity

	// 玩家名
	GetName() string

	// 账号id
	GetAccountId() int64

	// 区服id
	GetRegionId() int32

	Send(command gnet.PacketCommand, message proto.Message, opts ...gnet.SendOption) bool

	SendPacket(packet gnet.Packet, opts ...gnet.SendOption) bool
}

type PlayerMgr interface {
	GetPlayer(playerId int64) Player
	AddPlayer(player Player)
	RemovePlayer(player Player)
}
