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
	registerPlayerComponentCtor(ComponentNameBaseInfo, 0, func(player *Player, playerData *pb.PlayerData) gentity.Component {
		component := &BaseInfo{
			DataComponent: *gentity.NewDataComponent(player, ComponentNameBaseInfo),
			BaseInfo: &pb.BaseInfo{
				Level:             1,
				Exp:               0,
				LongFieldNameTest: "HelloWorld",
			},
		}
		gentity.LoadData(component, playerData.GetBaseInfo())
		return component
	})
}

// 基本信息组件
type BaseInfo struct {
	gentity.DataComponent
	BaseInfo *pb.BaseInfo `db:"plain"`
}

func (this *Player) GetBaseInfo() *BaseInfo {
	return this.GetComponentByName(ComponentNameBaseInfo).(*BaseInfo)
}

func (this *BaseInfo) AddExp(exp int32) {
	gentity.Set(this, &this.BaseInfo.Exp, this.BaseInfo.Exp+exp)
	// gentity.Set等同于下面2行
	//this.BaseInfo.Exp += exp
	//this.SetDirty()
}

func (this *BaseInfo) SetLongFieldNameTest(str string) {
	gentity.SetFn(this, func() {
		this.BaseInfo.LongFieldNameTest = str
	})
	// gentity.SetFn等同于下面2行
	//this.BaseInfo.LongFieldNameTest = str
	//this.SetDirty()
}

// 组件上的事件响应接口
func (this *BaseInfo) TriggerLoopCheckA(evt *LoopCheckA) {
	gentity.GetLogger().Debug("BaseInfo.OnEventLoopCheckA:%v", evt)
	player := this.GetEntity().(*Player)
	// 在LoopCheckA的响应事件中,触发LoopCheckB事件
	player.FireEvent(&LoopCheckB{Name: "abcde"})
}

func (this *BaseInfo) TriggerLoopCheckB(evt *LoopCheckB) {
	gentity.GetLogger().Debug("BaseInfo.OnEventLoopCheckB:%v", evt)
	player := this.GetEntity().(*Player)
	// 在LoopCheckB的响应事件中,触发LoopCheckA事件
	player.FireEvent(&LoopCheckA{Num: 123})
}
