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
	registerPlayerComponentCtor(ComponentNameStruct, 0, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &StructComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameStruct),
		}
		gentity.LoadData(component, playerData.GetStruct())
		return component
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
