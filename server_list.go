package gentity

import (
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
)

type ServerList interface {
	SendToServer(serverId int32, cmd gnet.PacketCommand, message proto.Message) bool
}
