package examples

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gnet"
	"reflect"
)

// 玩家实体
type testPlayer struct {
	gentity.BaseEntity
	Id        int64  `json:"_id"`       // 玩家id
	Name      string `json:"name"`      // 玩家名
	AccountId int64  `json:"accountId"` // 账号id
	RegionId  int32  `json:"regionId"`  // 区服id
}

// 保存缓存
func (this *testPlayer) SaveCache(kvCache gentity.KvCache) error {
	return this.BaseEntity.SaveCache(kvCache, "p", this.GetId())
}

// 分发事件
func (this *testPlayer) FireEvent(event any) {
	hasHandler := _playerEventHandlerMgr.Invoke(this, event)
	if !hasHandler {
		gentity.GetLogger().Debug("no event handler:%v", reflect.TypeOf(event).String())
	}
}

func (this *testPlayer) RecvPacket(packet gnet.Packet) {
	_playerPacketHandlerMgr.Invoke(this, packet)
}

// entity上的消息回调接口
func (this *testPlayer) OnFinishQuestRes(reqCmd gnet.PacketCommand, req *pb.FinishQuestRes) {
	gentity.GetLogger().Debug("OnFinishQuestRes:%v", req)
}

// entity上的事件响应接口
func (this *testPlayer) OnEventPlayerEntryGame(evt *PlayerEntryGame) {
	gentity.GetLogger().Debug("testPlayer.OnEventPlayerEntryGame:%v", evt)
}

func newTestPlayer(playerId, accountId int64) *testPlayer {
	data := &pb.PlayerData{
		XId:       playerId,
		AccountId: accountId,
		Name:      fmt.Sprintf("player%v", playerId),
		RegionId:  1,
	}
	return newTestPlayerFromData(data)
}

func newTestPlayerFromData(data *pb.PlayerData) *testPlayer {
	p := &testPlayer{
		Id:        data.XId,
		AccountId: data.AccountId,
		Name:      data.Name,
		RegionId:  data.RegionId,
	}
	// 初始化组件
	_playerComponentRegister.InitComponents(p, data)
	return p
}

func getNewPlayerSaveData(p *testPlayer) map[string]interface{} {
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData["_id"] = p.Id
	newPlayerSaveData["name"] = p.Name
	newPlayerSaveData["accountid"] = p.AccountId
	newPlayerSaveData["regionid"] = p.RegionId
	gentity.GetEntitySaveData(p, newPlayerSaveData)
	return newPlayerSaveData
}
