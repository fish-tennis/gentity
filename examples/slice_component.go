package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

const (
	// 组件名
	ComponentNameSlice = "Slice"
)

// 利用go的init进行组件的自动注册
func init() {
	registerPlayerComponentCtor(ComponentNameSlice, 0, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &sliceComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameSlice),
		}
		gentity.LoadData(component, playerData.GetSlice())
		return component
	})
}

type sliceComponent struct {
	gentity.DataComponent
	Data []*pb.QuestData `db:""`
}

func (this *testPlayer) GetSlice() *sliceComponent {
	return this.GetComponentByName(ComponentNameSlice).(*sliceComponent)
}

func (this *sliceComponent) Add(data *pb.QuestData) {
	this.Data = append(this.Data, data)
	this.SetDirty()
}
