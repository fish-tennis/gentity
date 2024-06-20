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
			Finished:      &FinishedQuests{},
			Quests: &CurQuests{
				Quests: make(map[int32]*pb.QuestData),
			},
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
	Finished *FinishedQuests `child:""`
	// 保存数据的子模块:当前任务列表
	Quests *CurQuests `child:""`
}

func (this *testPlayer) GetQuest() *questComponent {
	return this.GetComponentByName(ComponentNameQuest).(*questComponent)
}

// 已完成的任务
type FinishedQuests struct {
	gentity.BaseDirtyMark
	// struct tag里面没有设置保存字段名,会默认使用字段名的全小写形式
	Finished []int32 `db:"plain"` // 基础类型,设置明文存储
}

func (f *FinishedQuests) Add(finishedQuestId int32) {
	if slices.Contains(f.Finished, finishedQuestId) {
		return
	}
	f.Finished = append(f.Finished, finishedQuestId)
	f.SetDirty()
}

// 当前任务列表
type CurQuests struct {
	gentity.BaseMapDirtyMark
	// struct tag里面没有设置保存字段名,会默认使用字段名的全小写形式
	Quests map[int32]*pb.QuestData `db:""`
}

func (c *CurQuests) Add(questData *pb.QuestData) {
	if c.Quests == nil {
		c.Quests = make(map[int32]*pb.QuestData)
	}
	gentity.MapAdd(c, c.Quests, questData.CfgId, questData)
}

func (c *CurQuests) Remove(questId int32) {
	gentity.MapDel(c, c.Quests, questId)
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
