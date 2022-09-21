package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
)

// 任务组件
type questComponent struct {
	gentity.BaseComponent
	// 保存数据的子模块:已完成的任务
	// 保存数据的子模块必须是导出字段(字段名大写开头)
	Finished *FinishedQuests `child:""`
	// 保存数据的子模块:当前任务列表
	Quests *CurQuests `child:""`
}

// 已完成的任务
type FinishedQuests struct {
	gentity.BaseDirtyMark
	// struct tag里面没有设置保存字段名,会默认使用字段名的全小写形式
	Finished []int32 `db:"plain"` // 基础类型,设置明文存储
}

func (f *FinishedQuests) Add(finishedQuestId int32) {
	if util.ContainsInt32(f.Finished, finishedQuestId) {
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
	c.Quests[questData.CfgId] = questData
	c.SetDirty(questData.CfgId, true)
}

func (c *CurQuests) Remove(questId int32) {
	delete(c.Quests, questId)
	c.SetDirty(questId, false)
}

// 完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *questComponent) OnFinishQuestReq(reqCmd gnet.PacketCommand, req *pb.FinishQuestReq) {
	gentity.GetLogger().Debug("OnFinishQuestReq:%v", req)
}