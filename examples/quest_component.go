package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gnet"
	"slices"
)

const (
	// 组件名
	ComponentNameQuest = "Quest"
)

// 利用go的init进行组件的自动注册
func init() {
	registerPlayerComponentCtor(ComponentNameQuest, 0, func(player *testPlayer, playerData *pb.PlayerData) gentity.Component {
		component := &questComponent{
			BaseComponent: *gentity.NewBaseComponent(player, ComponentNameQuest),
			Finished:      new(gentity.SliceData[int32]),
			Quests:        gentity.NewMapData[int32, *pb.QuestData](),
		}
		gentity.LoadData(component, playerData.GetQuest())
		return component
	})
}

// 任务组件
type questComponent struct {
	gentity.BaseComponent
	// 保存数据的子模块:已完成的任务
	// 保存数据的子模块必须是导出字段(字段名大写开头)
	Finished *gentity.SliceData[int32] `child:"plain"` // 明文保存
	// 保存数据的子模块:当前任务列表
	Quests *gentity.MapData[int32, *pb.QuestData] `child:""`
}

func (this *testPlayer) GetQuest() *questComponent {
	return this.GetComponentByName(ComponentNameQuest).(*questComponent)
}

func (this *questComponent) AddFinishId(id int32) {
	if slices.Contains(this.Finished.Data, id) {
		return
	}
	this.Finished.Add(id)
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *questComponent) OnFinishQuestReq(reqCmd gnet.PacketCommand, req *pb.FinishQuestReq) {
	gentity.GetLogger().Debug("OnFinishQuestReq:%v", req)
}

// 组件上的事件响应接口
func (this *questComponent) TriggerPlayerEntryGame(evt *PlayerEntryGame) {
	gentity.GetLogger().Debug("questComponent.OnEventPlayerEntryGame:%v", evt)
}
