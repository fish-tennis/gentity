package examples

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

var (
	_mongoUri       = "mongodb://localhost:27017"
	_mongoDbName    = "test"
	_collectionName = "player"
	_redisAddrs     = []string{"127.0.0.1:6379"}
	_redisUsername  = ""
	_redisPassword  = ""
	// 如果部署的是单机版redis,则需要修改为false
	_isRedisCluster = true
)

func initRedis() gentity.KvCache {
	var redisCmdable redis.Cmdable
	if _isRedisCluster {
		redisCmdable = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    _redisAddrs,
			Username: _redisUsername,
			Password: _redisPassword,
		})
	} else {
		redisCmdable = redis.NewClient(&redis.Options{
			Addr:     _redisAddrs[0],
			Username: _redisUsername,
			Password: _redisPassword,
		})
	}
	pong, err := redisCmdable.Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic(fmt.Sprintf("redis connect error:%v", err.Error()))
	}
	return gentity.NewRedisCache(redisCmdable)
}

// 测试根据账号查找角色的接口
func TestFindPlayerId(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	playerDb.DeleteEntity(1)
	player1 := newTestPlayer(103, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	// 适合一个服只有一个角色的应用场景,如竞技游戏
	findPlayerId, err := playerDb.FindPlayerIdByAccountId(player1.AccountId, player1.RegionId)
	if err != nil {
		t.Log(err)
	}
	t.Logf("findPlayerId:%v", findPlayerId)

	// 适合一个服有多个角色的应用场景,如多角色MMORPG
	findPlayerIds, err := playerDb.FindPlayerIdsByAccountId(player1.AccountId, player1.RegionId)
	if err != nil {
		t.Log(err)
	}
	t.Logf("findPlayerIds:%v", findPlayerIds)

	// 新建3个角色数据
	for i := 0; i < 3; i++ {
		playerDb.DeleteEntity(int64(100 + i))
		playeri := newTestPlayer(int64(100+i), 100)
		playerDb.InsertEntity(playeri.Id, getNewPlayerSaveData(playeri))
	}
	// 一个账号下的角色列表
	findPlayerIds, err = playerDb.FindPlayerIdsByAccountId(100, 1)
	if err != nil {
		t.Log(err)
	}
	t.Logf("findPlayerIds:%v", findPlayerIds)
}

// 测试缓存接口
func TestDbCache(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	playerDb.DeleteEntity(1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetBaseInfo()
	baseInfo.AddExp(456)
	// 只会把baseinfo组件保存到缓存
	player1.SaveCache(kvCache)

	quest := player1.GetQuest()
	quest.AddFinishId(1)
	questData2 := &pb.QuestData{
		CfgId:    2,
		Progress: 5,
	}
	quest.Quests.Set(questData2.CfgId, questData2)
	questData3 := &pb.QuestData{
		CfgId:    3,
		Progress: 6,
	}
	quest.Quests.Set(questData3.CfgId, questData3)
	// 只会把quest组件保存到缓存
	player1.SaveCache(kvCache)

	interfaceMap := player1.GetInterfaceMap()
	if len(interfaceMap.InterfaceMap.Data) == 0 {
		interfaceMap.makeTestData()
	}
	item1 := interfaceMap.InterfaceMap.Data["mapItem1"].(*mapItem1)
	item1.addExp(10)
	player1.SaveCache(kvCache)

	array := player1.GetArray()
	for i := 0; i < len(array.Array); i++ {
		array.Array[i] = int32(i) + 1
	}
	array.SetDirty()
	player1.SaveCache(kvCache)

	slice := player1.GetSlice()
	slice.Add(&pb.QuestData{CfgId: 4, Progress: 7})
	slice.Add(&pb.QuestData{CfgId: 5, Progress: 8})
	player1.SaveCache(kvCache)

	player1.GetStruct().Set(11, 12)
	player1.SaveCache(kvCache)

	bag := player1.GetBag()
	for i := 0; i < 3; i++ {
		bag.BagCountItem.AddItem(int32(i+1), int32((i+1)*10))
		bag.BagUniqueItem.AddUniqueItem(&pb.UniqueItem{
			UniqueId: time.Now().Unix() + int64(i*1000),
			CfgId:    int32(i+1) + int32(1000),
		})
		bag.TestUniqueItem.Add(&pb.UniqueItem{
			UniqueId: time.Now().Unix() + int64(i*10000),
			CfgId:    int32(i+1) + int32(10000),
		})
	}
	player1.SaveCache(kvCache)

	// 只会把修改过数据的组件更新到数据库
	gentity.SaveEntityChangedDataToDb(playerDb, player1, kvCache, true, "p")

	bytes, err := gentity.GetSaveData(player1.GetStruct(), "")
	if err == nil {
		testData := pb.QuestData{}
		err = proto.Unmarshal(bytes.([]byte), &testData)
		t.Logf("err:%v testData:%v", err, &testData)
	}

	loadData := &pb.PlayerData{}
	playerDb.FindEntityById(player1.Id, loadData)
	t.Logf("loadData:%v", loadData)
	t.Logf("loadData.Struct:%v", loadData.Struct)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("BaseInfo:%v", loadPlayer.GetBaseInfo())
	t.Logf("Quest.Finished:%v", loadPlayer.GetQuest().Finished.Data)
	t.Logf("Quest.Quests:%v", loadPlayer.GetQuest().Quests.Data)
	t.Logf("InterfaceMap:%v", loadPlayer.GetInterfaceMap().InterfaceMap)
	t.Logf("Array:%v", loadPlayer.GetArray().Array)
	t.Logf("Slice:%v", loadPlayer.GetSlice().Data)
	s := loadPlayer.GetStruct()
	t.Logf("Struct:%v", &s.Data)
	t.Logf("Bag.CountItem:%v", loadPlayer.GetBag().BagCountItem.Data)
	t.Logf("Bag.UniqueItem:%v", loadPlayer.GetBag().BagUniqueItem.Data)
	t.Logf("Bag.TestUniqueItem:%v", loadPlayer.GetBag().TestUniqueItem.Data)
}

// 测试从缓存修复数据的接口
func TestFixDataFromCache(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	playerDb.DeleteEntity(1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetBaseInfo()
	baseInfo.AddExp(123)
	baseInfo.SetLongFieldNameTest("FixDataFromCacheTest")
	player1.SaveCache(kvCache)

	quest := player1.GetQuest()
	quest.AddFinishId(1)
	questData2 := &pb.QuestData{
		CfgId:    2,
		Progress: 5,
	}
	quest.Quests.Set(questData2.CfgId, questData2)
	player1.SaveCache(kvCache)

	interfaceMap := player1.GetInterfaceMap()
	if len(interfaceMap.InterfaceMap.Data) == 0 {
		interfaceMap.makeTestData()
	}
	item1 := interfaceMap.InterfaceMap.Data["mapItem1"].(*mapItem1)
	item1.addExp(10)
	player1.SaveCache(kvCache)

	array := player1.GetArray()
	for i := 0; i < len(array.Array); i++ {
		array.Array[i] = int32(i) + 1
	}
	array.SetDirty()
	player1.SaveCache(kvCache)

	slice := player1.GetSlice()
	slice.Add(&pb.QuestData{CfgId: 4, Progress: 7})
	slice.Add(&pb.QuestData{CfgId: 5, Progress: 8})
	player1.SaveCache(kvCache)

	bag := player1.GetBag()
	for i := 0; i < 3; i++ {
		bag.BagCountItem.AddItem(int32(i+1), int32((i+1)*10))
		bag.BagUniqueItem.AddUniqueItem(&pb.UniqueItem{
			UniqueId: time.Now().Unix() + int64(i*1000),
			CfgId:    int32(i+1) + int32(1000),
		})
		bag.TestUniqueItem.Add(&pb.UniqueItem{
			UniqueId: time.Now().Unix() + int64(i*10000),
			CfgId:    int32(i+1) + int32(10000),
		})
	}
	player1.SaveCache(kvCache)

	fixPlayer := newTestPlayer(1, 1)
	// 上面player1的修改数据之保存到了缓存,并没有保存到数据库
	// 所以这里模拟了player1的修改数据没保存到数据库的情景
	// 调用FixEntityDataFromCache,将会把缓存里的数据同步到数据库
	gentity.FixEntityDataFromCache(fixPlayer, playerDb, kvCache, "p", fixPlayer.GetId())

	loadData := &pb.PlayerData{}
	playerDb.FindEntityById(player1.Id, loadData)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("BaseInfo:%v", loadPlayer.GetBaseInfo())
	t.Logf("Quest.Finished:%v", loadPlayer.GetQuest().Finished.Data)
	t.Logf("Quest.Quests:%v", loadPlayer.GetQuest().Quests.Data)
	t.Logf("InterfaceMap:%v", loadPlayer.GetInterfaceMap().InterfaceMap)
	for k, v := range loadPlayer.GetInterfaceMap().InterfaceMap.Data {
		t.Logf("%v:%v", k, v)
	}
	t.Logf("Array:%v", loadPlayer.GetArray().Array)
	t.Logf("Slice:%v", loadPlayer.GetSlice().Data)
	s := loadPlayer.GetStruct()
	t.Logf("Struct:%v", &s.Data)
	t.Logf("Bag.CountItem:%v", loadPlayer.GetBag().BagCountItem.Data)
	t.Logf("Bag.UniqueItem:%v", loadPlayer.GetBag().BagUniqueItem.Data)
	t.Logf("Bag.TestUniqueItem:%v", loadPlayer.GetBag().TestUniqueItem.Data)
}

// 测试自动注册
func TestHandlerRegister(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	// 注册消息回调接口和事件响应接口
	autoRegisterTestPlayer()
	player := newTestPlayer(0, 0)
	// 模拟玩家分发一个事件
	player.FireEvent(&PlayerEntryGame{
		IsReconnect:    true,
		OfflineSeconds: 12345,
	})
	// 模拟一个嵌套事件
	player.FireEvent(&LoopCheckB{
		Name: "start loop",
	})
}

func TestPlayerData(t *testing.T) {
	gentity.SetLogLevel(gentity.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	playerId := int64(103)
	playerData := &pb.PlayerData{XId: playerId}
	exists, err := playerDb.FindEntityById(playerId, playerData)
	if err != nil {
		t.Fatal(fmt.Sprintf("%v", err))
	}
	if !exists {
		newPlayer := newTestPlayer(playerId, 1)
		playerDb.InsertEntity(newPlayer.Id, getNewPlayerSaveData(newPlayer))
		t.Logf("InsertPlayer %v", newPlayer.Id)
	} else {
		newPlayer := newTestPlayerFromData(playerData)
		t.Logf("LoadPlayer %v", newPlayer.Id)
	}
}
