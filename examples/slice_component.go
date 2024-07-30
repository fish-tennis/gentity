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
	registerPlayerComponentCtor(ComponentNameSlice, 0, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &SliceComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameSlice),
		}
		gentity.LoadData(component, playerData.GetSlice())
		return component
	})
}

type SliceComponent struct {
	gentity.DataComponent
	Data []*pb.QuestData `db:""`
}

func (this *Player) GetSlice() *SliceComponent {
	return this.GetComponentByName(ComponentNameSlice).(*SliceComponent)
}

func (this *SliceComponent) Add(data *pb.QuestData) {
	this.Data = append(this.Data, data)
	this.SetDirty()
}
