package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

const (
	// 组件名
	ComponentNameStruct = "Struct"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameStruct, 0, func(player *Player, _ any) gentity.Component {
		return &StructComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameStruct),
		}
	})
}

type StructComponent struct {
	gentity.DataComponent
	Data pb.QuestData `db:""`
}

func (this *Player) GetStruct() *StructComponent {
	return this.GetComponentByName(ComponentNameStruct).(*StructComponent)
}

func (this *StructComponent) Set(cfgId, progress int32) {
	this.Data.CfgId = cfgId
	this.Data.Progress = progress
	this.SetDirty()
}
