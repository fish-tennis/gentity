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
	registerPlayerComponentCtor(ComponentNameStruct, 0, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &structComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameStruct),
		}
		gentity.LoadData(component, playerData.GetStruct())
		return component
	})
}

type structComponent struct {
	gentity.DataComponent
	Data pb.QuestData `db:""`
}

func (this *testPlayer) GetStruct() *structComponent {
	return this.GetComponentByName(ComponentNameStruct).(*structComponent)
}

func (this *structComponent) Set(cfgId, progress int32) {
	this.Data.CfgId = cfgId
	this.Data.Progress = progress
	this.SetDirty()
}
