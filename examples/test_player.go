package examples

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
)

// 玩家实体
type testPlayer struct {
	gentity.BaseEntity
	Id        int64  `json:"_id"`        // 玩家id
	Name      string `json:"name"`      // 玩家名
	AccountId int64  `json:"accountId"` // 账号id
	RegionId  int32  `json:"regionId"`  // 区服id
}

func (this *testPlayer) GetId() int64 {
	return this.Id
}

func newTestPlayer(playerId, accountId int64) *testPlayer {
	data := &pb.PlayerData{
		XId: playerId,
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
	p.AddComponent(&baseInfoComponent{
		DataComponent: *gentity.NewDataComponent(p, "baseinfo"),
		BaseInfo: &pb.BaseInfo{
			Gender: 1,
			Level:  1,
			Exp:    0,
		},
	}, data.BaseInfo)
	p.AddComponent(&questComponent{
		BaseComponent: *gentity.NewBaseComponent(p, "quest"),
		Finished: &FinishedQuests{
			Finished: make([]int32, 0),
		},
		Quests: &CurQuests{
			Quests: make(map[int32]*pb.QuestData, 0),
		},
	}, data.Quest)
	im := newInterfaceMapComponent(p)
	im.loadData(data.InterfaceMap)
	p.AddComponent(im, nil)
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
