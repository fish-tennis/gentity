package examples

import (
	"context"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gnet"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

var (
	_mongoUri    = "mongodb://localhost:27017"
	_mongoDbName = "test"
	_collectionName = "player"
	_redisAddrs = []string{"127.0.0.1:6379"}
	_redisPassword = ""
	// 如果部署的是单机版redis,则需要修改为false
	_isRedisCluster = true
)

func initRedis() gentity.KvCache {
	var redisCmdable redis.Cmdable
	if _isRedisCluster {
		redisCmdable = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:_redisAddrs,
			Password: _redisPassword,
		})
	} else {
		redisCmdable = redis.NewClient(&redis.Options{
			Addr:_redisAddrs[0],
			Password: _redisPassword,
		})
	}
	pong, err := redisCmdable.Ping(context.Background()).Result()
	if err != nil || pong == "" {
		panic("redis connect error")
	}
	return gentity.NewRedisCache(redisCmdable)
}

// 测试根据账号查找角色的接口
func TestFindPlayerId(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerPb(_collectionName, "id", "name", "accountid", "regionid")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	deletePlayer(mongoDb,1)
	player1 := newTestPlayer(1, 1)
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
		deletePlayer(mongoDb,int64(100 + i))
		playeri := newTestPlayer(int64(100 + i), 100)
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
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerPb(_collectionName, "id", "name", "accountid", "regionid")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	deletePlayer(mongoDb,1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetComponentByName("baseinfo").(*baseInfoComponent)
	baseInfo.AddExp(123)
	// 只会把baseinfo组件保存到缓存
	player1.SaveCache(kvCache)

	quest := player1.GetComponentByName("quest").(*questComponent)
	quest.Finished.Add(1)
	quest.Quests.Add(&pb.QuestData{
		CfgId: 2,
		Progress: 5,
	})
	// 只会把quest组件保存到缓存
	player1.SaveCache(kvCache)

	//time.Sleep(time.Second*3)
	// 只会把修改过数据的组件更新到数据库
	gentity.SaveEntityChangedDataToDb(playerDb, player1, kvCache, true)

	loadData := &pb.PlayerData{}
	playerDb.FindEntityById(player1.Id, loadData)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("%v", loadPlayer.GetComponentByName("baseinfo").(*baseInfoComponent))
	t.Logf("%v", loadPlayer.GetComponentByName("quest").(*questComponent).Finished.Finished)
	t.Logf("%v", loadPlayer.GetComponentByName("quest").(*questComponent).Quests.Quests)
}

// 测试从缓存修复数据的接口
func TestFixDataFromCache(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerPb(_collectionName, "id", "name", "accountid", "regionid")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	kvCache := initRedis()

	deletePlayer(mongoDb,1)
	player1 := newTestPlayer(1, 1)
	playerDb.InsertEntity(player1.Id, getNewPlayerSaveData(player1))

	baseInfo := player1.GetComponentByName("baseinfo").(*baseInfoComponent)
	baseInfo.AddExp(123)
	player1.SaveCache(kvCache)

	quest := player1.GetComponentByName("quest").(*questComponent)
	quest.Finished.Add(1)
	quest.Quests.Add(&pb.QuestData{
		CfgId: 2,
		Progress: 5,
	})
	player1.SaveCache(kvCache)

	fixPlayer := newTestPlayer(1, 1)
	// 上面player1的修改数据之保存到了缓存,并没有保存到数据库
	// 所以这里模拟了player1的修改数据没保存到数据库的情景
	// 调用FixEntityDataFromCache,将会把缓存里的数据同步到数据库
	gentity.FixEntityDataFromCache(fixPlayer, playerDb, kvCache, "p")

	loadData := &pb.PlayerData{}
	playerDb.FindEntityById(player1.Id, loadData)
	loadPlayer := newTestPlayerFromData(loadData)
	t.Logf("%v", loadPlayer.GetComponentByName("baseinfo").(*baseInfoComponent))
	t.Logf("%v", loadPlayer.GetComponentByName("quest").(*questComponent).Finished.Finished)
	t.Logf("%v", loadPlayer.GetComponentByName("quest").(*questComponent).Quests.Quests)
}

// 测试消息自动注册
func TestHandlerRegister(t *testing.T) {
	gnet.SetLogLevel(gnet.DebugLevel)
	gentity.SetLogger(gnet.GetLogger())
	tmpPlayer := newTestPlayer(0,0)
	connectionHandler := gnet.NewDefaultConnectionHandler(nil)
	// 扫描注册消息
	gentity.AutoRegisterComponentHandler(tmpPlayer, connectionHandler, "On", "Handle", "gserver" )
	// 模拟一次消息调用
	gentity.ProcessComponentHandler(tmpPlayer, gnet.PacketCommand(pb.CmdQuest_Cmd_FinishQuestReq), &pb.FinishQuestReq{
		QuestCfgId: 123,
	})
}