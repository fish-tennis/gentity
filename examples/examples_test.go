package examples

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gnet"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/protobuf/proto"
	"testing"
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
	gnet.SetLogLevel(gnet.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	deletePlayer(mongoDb, 1)
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
		deletePlayer(mongoDb, int64(100+i))
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

func deletePlayer(mongoDb *gentity.MongoDb, id int64) {
	mongoDb.GetMongoDatabase().Collection(_collectionName).FindOneAndDelete(context.Background(), bson.D{{"id", id}})
}

// 测试缓存接口
func TestDbCache(t *testing.T) {
	gnet.SetLogLevel(gnet.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	deletePlayer(mongoDb, 1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetBaseInfo()
	baseInfo.AddExp(123)
	// 只会把baseinfo组件保存到缓存
	player1.SaveCache(kvCache)

	quest := player1.GetQuest()
	quest.Finished.Add(1)
	quest.Quests.Add(&pb.QuestData{
		CfgId:    2,
		Progress: 5,
	})
	quest.Quests.Add(&pb.QuestData{
		CfgId:    3,
		Progress: 6,
	})
	// 只会把quest组件保存到缓存
	player1.SaveCache(kvCache)

	interfaceMap := player1.GetInterfaceMap()
	interfaceMap.InterfaceMap["item1"].(*item1).addExp(10)
	interfaceMap.SetDirty("item1", true)
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
	//// loadData.Struct的值未加载进来
	t.Logf("loadData:%v", loadData)
	t.Logf("loadData.Struct:%v", loadData.Struct)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("BaseInfo:%v", loadPlayer.GetBaseInfo())
	t.Logf("Quest.Finished:%v", loadPlayer.GetQuest().Finished.Finished)
	t.Logf("Quest.Quests:%v", loadPlayer.GetQuest().Quests.Quests)
	t.Logf("InterfaceMap:%v", loadPlayer.GetInterfaceMap().InterfaceMap)
	t.Logf("Array:%v", loadPlayer.GetArray().Array)
	t.Logf("Slice:%v", loadPlayer.GetSlice().Data)
	s := loadPlayer.GetStruct()
	t.Logf("Struct:%v", &s.Data)
}

// 测试从缓存修复数据的接口
func TestFixDataFromCache(t *testing.T) {
	gnet.SetLogLevel(gnet.DebugLevel)
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb(_collectionName, true, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	deletePlayer(mongoDb, 1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetBaseInfo()
	baseInfo.AddExp(123)
	baseInfo.SetLongFieldNameTest("FixDataFromCacheTest")
	player1.SaveCache(kvCache)

	quest := player1.GetQuest()
	quest.Finished.Add(1)
	quest.Quests.Add(&pb.QuestData{
		CfgId:    2,
		Progress: 5,
	})
	player1.SaveCache(kvCache)

	interfaceMap := player1.GetInterfaceMap()
	interfaceMap.InterfaceMap["item1"].(*item1).addExp(10)
	interfaceMap.SetDirty("item1", true)
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

	fixPlayer := newTestPlayer(1, 1)
	// 上面player1的修改数据之保存到了缓存,并没有保存到数据库
	// 所以这里模拟了player1的修改数据没保存到数据库的情景
	// 调用FixEntityDataFromCache,将会把缓存里的数据同步到数据库
	gentity.FixEntityDataFromCache(fixPlayer, playerDb, kvCache, "p", fixPlayer.GetId())

	loadData := &pb.PlayerData{}
	playerDb.FindEntityById(player1.Id, loadData)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("BaseInfo:%v", loadPlayer.GetBaseInfo())
	t.Logf("Quest.Finished:%v", loadPlayer.GetQuest().Finished.Finished)
	t.Logf("Quest.Quests:%v", loadPlayer.GetQuest().Quests.Quests)
	t.Logf("InterfaceMap:%v", loadPlayer.GetInterfaceMap().InterfaceMap)
	t.Logf("Array:%v", loadPlayer.GetArray().Array)
	t.Logf("Slice:%v", loadPlayer.GetSlice().Data)
}

// 测试自动注册
func TestHandlerRegister(t *testing.T) {
	gnet.SetLogLevel(gnet.DebugLevel)
	gentity.SetLogger(gnet.GetLogger())
	// 注册消息回调接口和事件响应接口
	autoRegisterTestPlayer()
	player := newTestPlayer(0, 0)
	// 模拟玩家收到一个网络消息
	player.RecvPacket(gnet.NewProtoPacketEx(pb.CmdQuest_Cmd_FinishQuestReq, &pb.FinishQuestReq{
		QuestCfgId: 123,
	}))
	// 模拟玩家收到一个网络消息
	player.RecvPacket(gnet.NewProtoPacketEx(pb.CmdQuest_Cmd_FinishQuestRes, &pb.FinishQuestRes{
		QuestCfgId: 456,
	}))
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
	gnet.SetLogLevel(gnet.DebugLevel)
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
