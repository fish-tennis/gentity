package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

const (
	// 组件名
	ComponentNameBaseInfo = "BaseInfo"
)

// 利用go的init进行组件的自动注册
func init() {
	registerPlayerComponentCtor(ComponentNameBaseInfo, 0, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &baseInfoComponent{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameBaseInfo),
			BaseInfo: &pb.BaseInfo{
				Level: 1,
				Exp:   0,
			},
		}
		gentity.LoadData(component, playerData.GetBaseInfo())
		return component
	})
}

// 基本信息组件
type baseInfoComponent struct {
	gentity.DataComponent
	BaseInfo *pb.BaseInfo `db:"plain"`
}

func (this *testPlayer) GetBaseInfo() *baseInfoComponent {
	return this.GetComponentByName(ComponentNameBaseInfo).(*baseInfoComponent)
}

func (this *baseInfoComponent) AddExp(exp int32) {
	this.BaseInfo.Exp += exp
	this.SetDirty()
}
