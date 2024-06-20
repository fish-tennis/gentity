package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

const (
	// 组件名
	ComponentNameArray = "Array"
)

// 利用go的init进行组件的自动注册
func init() {
	registerPlayerComponentCtor(ComponentNameArray, 0, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &arrayComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameArray),
		}
		gentity.LoadData(component, playerData.GetArray())
		return component
	})
}

// 固定长度数组测试
type arrayComponent struct {
	gentity.DataComponent
	Array [10]int32 `db:"plain"`
}

func (this *testPlayer) GetArray() *arrayComponent {
	return this.GetComponentByName(ComponentNameArray).(*arrayComponent)
}
