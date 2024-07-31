package examples

import (
	"github.com/fish-tennis/gentity"
)

const (
	// 组件名
	ComponentNameArray = "Array"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameArray, 0, func(player *Player, _ any) gentity.Component {
		return &ArrayComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameArray),
		}
	})
}

// 固定长度数组测试
type ArrayComponent struct {
	gentity.DataComponent
	Array [10]int32 `db:"plain"`
}

func (this *Player) GetArray() *ArrayComponent {
	return this.GetComponentByName(ComponentNameArray).(*ArrayComponent)
}
