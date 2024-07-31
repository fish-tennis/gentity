package examples

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"reflect"
)

// 玩家实体
type Player struct {
	gentity.BaseEntity
	Name      string `json:"Name"`      // 玩家名
	AccountId int64  `json:"AccountId"` // 账号id
	RegionId  int32  `json:"RegionId"`  // 区服id
	// 事件分发的嵌套检测
	fireEventLoopChecker map[reflect.Type]int32
}

// 保存缓存
func (this *Player) SaveCache(kvCache gentity.KvCache) error {
	return this.BaseEntity.SaveCache(kvCache, "p", this.GetId())
}

// 分发事件
func (this *Player) FireEvent(event any) {
	if this.fireEventLoopChecker == nil {
		this.fireEventLoopChecker = make(map[reflect.Type]int32)
	}
	// 嵌套检测
	this.fireEventLoopChecker[reflect.TypeOf(event)]++
	defer func() {
		this.fireEventLoopChecker[reflect.TypeOf(event)]--
	}()
	if this.fireEventLoopChecker[reflect.TypeOf(event)] > 1 {
		gentity.GetLogger().Warn("FireEventLoopChecker depth:%v event:%v", this.fireEventLoopChecker, reflect.TypeOf(event).String())
		if this.fireEventLoopChecker[reflect.TypeOf(event)] > _fireSameEventLoopLimit {
			gentity.GetLogger().Error("FireEvent stop, limit:%v event:%v", _fireSameEventLoopLimit, reflect.TypeOf(event).String())
			return
		}
	}
	hasHandler := _playerEventHandlerMgr.Invoke(this, event)
	if !hasHandler {
		gentity.GetLogger().Debug("no event handler:%v", reflect.TypeOf(event).String())
	}
}

//// entity上的消息回调接口
//func (this *Player) OnFinishQuestRes(reqCmd gnet.PacketCommand, req *pb.FinishQuestRes) {
//	gentity.GetLogger().Debug("OnFinishQuestRes:%v", req)
//}

// entity上的事件响应接口
func (this *Player) TriggerPlayerEntryGame(evt *PlayerEntryGame) {
	gentity.GetLogger().Debug("Player.OnEventPlayerEntryGame:%v", evt)
}

func newTestPlayer(playerId, accountId int64) *Player {
	data := &pb.PlayerData{
		XId:       playerId,
		AccountId: accountId,
		Name:      fmt.Sprintf("player%v", playerId),
		RegionId:  1,
	}
	return newTestPlayerFromData(data)
}

func newTestPlayerFromData(data *pb.PlayerData) *Player {
	p := &Player{
		AccountId: data.AccountId,
		Name:      data.Name,
		RegionId:  data.RegionId,
	}
	p.Id = data.XId
	// 初始化组件
	_playerComponentRegister.InitComponents(p, data)
	if data.BaseInfo != nil {
		gentity.LoadEntityData(p, data)
	}
	return p
}

func getNewPlayerSaveData(p *Player) map[string]interface{} {
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData["_id"] = p.Id
	newPlayerSaveData["Name"] = p.Name
	newPlayerSaveData["AccountId"] = p.AccountId
	newPlayerSaveData["RegionId"] = p.RegionId
	gentity.GetEntitySaveData(p, newPlayerSaveData)
	return newPlayerSaveData
}
