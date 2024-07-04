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
type baseInfoComponent struct {
	gentity.DataComponent
	BaseInfo *pb.BaseInfo `db:"plain"`
}

func (this *testPlayer) GetBaseInfo() *baseInfoComponent {
	return this.GetComponentByName(ComponentNameBaseInfo).(*baseInfoComponent)
}

func (this *baseInfoComponent) AddExp(exp int32) {
	gentity.Set(this, &this.BaseInfo.Exp, this.BaseInfo.Exp+exp)
	// gentity.Set等同于下面2行
	//this.BaseInfo.Exp += exp
	//this.SetDirty()
}

func (this *baseInfoComponent) SetLongFieldNameTest(str string) {
	gentity.SetFn(this, func() {
		this.BaseInfo.LongFieldNameTest = str
	})
	// gentity.SetFn等同于下面2行
	//this.BaseInfo.LongFieldNameTest = str
	//this.SetDirty()
}

// 组件上的事件响应接口
func (this *baseInfoComponent) TriggerLoopCheckA(evt *LoopCheckA) {
	gentity.GetLogger().Debug("baseInfoComponent.OnEventLoopCheckA:%v", evt)
	player := this.GetEntity().(*testPlayer)
	// 在LoopCheckA的响应事件中,触发LoopCheckB事件
	player.FireEvent(&LoopCheckB{Name: "abcde"})
}

func (this *baseInfoComponent) TriggerLoopCheckB(evt *LoopCheckB) {
	gentity.GetLogger().Debug("baseInfoComponent.OnEventLoopCheckB:%v", evt)
	player := this.GetEntity().(*testPlayer)
	// 在LoopCheckB的响应事件中,触发LoopCheckA事件
	player.FireEvent(&LoopCheckA{Num: 123})
}
