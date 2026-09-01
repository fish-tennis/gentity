package examples

import (
	"context"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gentity/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sync"
	"testing"
)

// mongo实现的自增id方式
func TestIncrementId(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	kvDb := mongoDb.RegisterKvDb("kv", gentity.ShardKeyNone, "k", "v")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := kvDb.Inc("id", 1, true)
			if err != nil {
				t.Logf("%v", err)
			}
			t.Logf("%v", val)
		}()
	}
	wg.Wait()
}

func TestKvDb(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	kvDb := mongoDb.RegisterKvDb("kv", gentity.ShardKeyNone, "k", "v")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	protoData := &pb.BaseInfo{
		Gender: 1,
		Level:  10,
		Exp:    123,
	}
	kvDb.Insert("start_timestamp", util.GetCurrentTimeStamp())
	kvDb.Update("current_ms", util.GetCurrentMS(), true)
	kvDb.Update("proto_data", protoData, true)
	kvDb.Insert("temp", "temp value")
	start_timestamp, err1 := kvDb.Find("start_timestamp")
	t.Logf("%v %v", start_timestamp, err1)
	current_ms, err2 := kvDb.Find("current_ms")
	t.Logf("%v %v", current_ms, err2)
	proto_data, err3 := kvDb.Find("proto_data")
	t.Logf("%v %v", proto_data, err3)
	protoDecodeData := new(pb.BaseInfo)
	err4 := kvDb.FindAndDecode("proto_data", protoDecodeData)
	t.Logf("%v %v", protoDecodeData, err4)
	temp, err5 := kvDb.Find("temp")
	t.Logf("%v %v", temp, err5)
	kvDb.Delete("temp")
}

// 设置分片
// 只对集群模式的mongodb有效
func TestShard(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb("player", gentity.ShardKeyHashed, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	err := mongoDb.ShardDatabase(_mongoDbName)
	if err != nil {
		t.Logf("ShardDatabase err:%v", err)
	}
	playerDb.DeleteEntity(1)
	player1 := newTestPlayer(1, 1)
	err, _ = playerDb.InsertEntity(player1.GetId(), getNewPlayerSaveData(player1))
	if err != nil {
		t.Logf("InsertEntity err:%v", err)
	}
}

// TestShardKeyEntityDb 测试用的解码结构
// 数值字段统一int64(bson的int32/int64解码到int64总是安全)
type shardTestPlayerData struct {
	Id      int64                 `bson:"_id"`
	Account int64                 `bson:"AccountId"`
	Region  int64                 `bson:"RegionId"`
	Name    string                `bson:"Name"`
	Level   int64                 `bson:"Level"`
	Bag     *shardTestBagData     `bson:"Bag"`
	Mail    *shardTestMailData    `bson:"Mail"`
}

type shardTestBagData struct {
	Gold  *int64 `bson:"gold"`
	Level *int64 `bson:"level"`
}

type shardTestMailData struct {
	Unread *int64 `bson:"unread"`
}

// 验证ShardKeyEntityDb:分片键(AccountId)与uniqueId(_id)不同时的附加条件读写
// 单机mongo验证语义正确性;分片直达路由需分片集群环境
func TestShardKeyEntityDb(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	playerDb := mongoDb.RegisterPlayerDb("player_shardkey", gentity.ShardKeyNone, "_id", "AccountId", "RegionId")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()
	// 分片键列=AccountId,与uniqueId(_id)不同
	playerDb.(*gentity.MongoCollectionPlayer).SetShardKeyName("AccountId")

	shardDb, ok := playerDb.(gentity.ShardKeyEntityDb)
	if !ok {
		t.Fatal("MongoCollectionPlayer should implement ShardKeyEntityDb")
	}
	if shardDb.ShardKeyName() != "AccountId" {
		t.Fatalf("ShardKeyName=%v", shardDb.ShardKeyName())
	}

	const (
		accountId = int64(100)
		playerId  = int64(101)
	)
	// 清场
	shardDb.DeleteEntityWithShardKey(accountId, playerId)
	// 插入:文档本身携带分片键字段,无需附加
	if err, _ := playerDb.InsertEntity(playerId, bson.M{
		"_id": playerId, "AccountId": accountId, "RegionId": 1, "Name": "p1",
	}); err != nil {
		t.Fatalf("InsertEntity: %v", err)
	}

	// 分片键匹配→找到;分片键不匹配→找不到
	// NOTE:decode用struct而非bson.M:driver解码到interface{}时嵌套文档是bson.D,
	// bson.M断言会静默失败;指针字段用于区分"字段不存在"与"零值"
	var data shardTestPlayerData
	found, err := shardDb.FindEntityByIdWithShardKey(accountId, playerId, &data)
	if err != nil || !found || data.Name != "p1" {
		t.Fatalf("FindEntityByIdWithShardKey: found=%v err=%v data=%v", found, err, data)
	}
	found, _ = shardDb.FindEntityByIdWithShardKey(accountId+1, playerId, &data)
	if found {
		t.Fatal("mismatched shardKey should not find")
	}

	// 组件读写
	if err := shardDb.SaveComponentWithShardKey(accountId, playerId, "Bag", bson.M{"gold": 5}); err != nil {
		t.Fatalf("SaveComponentWithShardKey: %v", err)
	}
	if err := shardDb.SaveComponentFieldWithShardKey(accountId, playerId, "Bag", "level", 2); err != nil {
		t.Fatalf("SaveComponentFieldWithShardKey: %v", err)
	}
	if err := shardDb.SaveComponentsWithShardKey(accountId, playerId, map[string]interface{}{
		"Mail": bson.M{"unread": 3},
	}); err != nil {
		t.Fatalf("SaveComponentsWithShardKey: %v", err)
	}
	// NOTE:decode复用data变量时,driver不会重置文档中不存在的字段,每次断言前必须重置
	data = shardTestPlayerData{}
	found, _ = shardDb.FindEntityByIdWithShardKey(accountId, playerId, &data)
	if !found || data.Bag == nil || data.Bag.Gold == nil || *data.Bag.Gold != 5 ||
		data.Bag.Level == nil || *data.Bag.Level != 2 ||
		data.Mail == nil || data.Mail.Unread == nil || *data.Mail.Unread != 3 {
		t.Fatalf("components data err: %+v", data)
	}

	// 删除组件字段
	if err := shardDb.DeleteComponentFieldWithShardKey(accountId, playerId, "Bag", "gold"); err != nil {
		t.Fatalf("DeleteComponentFieldWithShardKey: %v", err)
	}
	data = shardTestPlayerData{}
	found, _ = shardDb.FindEntityByIdWithShardKey(accountId, playerId, &data)
	if !found {
		t.Fatal("should found")
	}
	if data.Bag == nil || data.Bag.Gold != nil {
		t.Fatalf("Bag.gold should be unset: %+v", data)
	}
	if data.Bag.Level == nil || *data.Bag.Level != 2 {
		t.Fatalf("Bag.level should remain: %+v", data)
	}

	// 全量保存
	if err := shardDb.SaveEntityWithShardKey(accountId, playerId, bson.D{
		{Key: "$set", Value: bson.D{{Key: "Level", Value: 9}}},
	}); err != nil {
		t.Fatalf("SaveEntityWithShardKey: %v", err)
	}
	// 未附加分片键的老方法仍可用(分片集群下退化为广播,单机无差别)
	found, err = playerDb.FindEntityById(playerId, &data)
	if err != nil || !found || data.Level != 9 {
		t.Fatalf("FindEntityById: found=%v err=%v data=%+v", found, err, data)
	}

	// 删除
	if err := shardDb.DeleteEntityWithShardKey(accountId, playerId); err != nil {
		t.Fatalf("DeleteEntityWithShardKey: %v", err)
	}
	found, _ = shardDb.FindEntityByIdWithShardKey(accountId, playerId, &data)
	if found {
		t.Fatal("should not found after delete")
	}
}

// explain结果的解码结构
// MongoDB 8.x的mongos explain:queryPlanner.winningPlan.shards是"参与执行计划的shard列表",
// 直达查询stage=SINGLE_SHARD且shards长度为1;广播(scatter-gather)stage=SHARD_MERGE且长度=集群shard总数
// (注:4.x的旧格式shards直接挂在queryPlanner下,8.x移入了winningPlan)
type explainQueryPlanner struct {
	QueryPlanner struct {
		WinningPlan struct {
			Stage  string `bson:"stage"`
			Shards bson.A `bson:"shards"`
		} `bson:"winningPlan"`
	} `bson:"queryPlanner"`
}

// TestShardKeyRoutingOnShardedCluster 分片集群(mongos)环境下的分片键路由验证:
// 附加AccountId条件的查询({_id,AccountId}与{AccountId,RegionId})直达单shard,
// 不附加分片键的查询({_id})广播所有shard——这正是ShardKeyEntityDb存在的价值
//
// 依赖docker/mongo_sharded环境(mongos占27017,初始化步骤见其docker-compose.yml),
// 单机mongo/环境未就绪时自动Skip;与单机版TestShardKeyEntityDb互补:
// 单机版验证读写语义,本测试验证查询路由
func TestShardKeyRoutingOnShardedCluster(t *testing.T) {
	const dbName = "gentity_shardtest"
	const collectionName = "player_shardkey"
	mongoDb := gentity.NewMongoDb(_mongoUri, dbName)
	if !mongoDb.Connect() {
		t.Skipf("mongo %v unavailable, skip", _mongoUri)
	}
	defer mongoDb.Disconnect()
	ctx := context.Background()
	client := mongoDb.GetMongoClient()

	// 环境判别:必须连到mongos且集群至少2个shard
	// 单机mongod的hello响应无msg字段,mongos返回msg=isdbgrid
	var hello bson.M
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err != nil {
		t.Skipf("hello err: %v", err)
	}
	if hello["msg"] != "isdbgrid" {
		t.Skipf("not a mongos(hello.msg=%v); start docker/mongo_sharded first", hello["msg"])
	}
	var listShards struct {
		Shards bson.A `bson:"shards"`
	}
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "listShards", Value: 1}}).Decode(&listShards); err != nil {
		t.Skipf("listShards err: %v", err)
	}
	shardCount := len(listShards.Shards)
	if shardCount < 2 {
		t.Skipf("shards=%v; run addShard steps in docker-compose.yml first", shardCount)
	}

	// 幂等清理:分片集群上dropDatabase会同时清理分片元数据,便于重复运行本测试
	if err := client.Database(dbName).Drop(ctx); err != nil {
		t.Logf("drop database: %v", err)
	}

	// 注册:分片键AccountId(hashed),uniqueId=_id,两者不同
	playerDb := mongoDb.RegisterPlayerDb(collectionName, gentity.ShardKeyHashed, "_id", "AccountId", "RegionId")
	playerDb.(*gentity.MongoCollectionPlayer).SetShardKeyName("AccountId")
	// enableSharding + shardCollection(AccountId:hashed)
	// NOTE:Shard()只在ShardDatabase时执行,所以SetShardKeyName须在此之前
	if err := mongoDb.ShardDatabase(dbName); err != nil {
		t.Fatalf("ShardDatabase: %v", err)
	}
	// 显式确认分片已生效(ShardDatabase内部忽略了单个Shard()的错误)
	var collInfo bson.M
	err := client.Database("config").Collection("collections").
		FindOne(ctx, bson.D{{Key: "_id", Value: dbName + "." + collectionName}}).Decode(&collInfo)
	if err == mongo.ErrNoDocuments {
		t.Fatalf("collection %v.%v not sharded; check ShardDatabase log", dbName, collectionName)
	} else if err != nil {
		t.Fatalf("query config.collections: %v", err)
	}

	// 插入若干玩家(不同accountId,hashed分布到各shard)
	const playerCount = 10
	for i := 1; i <= playerCount; i++ {
		if err, _ := playerDb.InsertEntity(int64(1000+i), bson.M{
			"_id": int64(1000 + i), "AccountId": int64(i), "RegionId": 1,
		}); err != nil {
			t.Fatalf("InsertEntity %v: %v", i, err)
		}
	}

	// explain查询,返回参与执行计划的shard数量
	plannedShardCount := func(filter bson.D) int {
		var res explainQueryPlanner
		if err := client.Database(dbName).RunCommand(ctx, bson.D{
			{Key: "explain", Value: bson.D{
				{Key: "find", Value: collectionName},
				{Key: "filter", Value: filter},
			}},
			{Key: "verbosity", Value: "queryPlanner"},
		}).Decode(&res); err != nil {
			t.Fatalf("explain: %v", err)
		}
		return len(res.QueryPlanner.WinningPlan.Shards)
	}

	const (
		accountId = int64(5)
		playerId  = int64(1005)
	)
	// 带{_id,AccountId}(FindEntityByIdWithShardKey的filter)→直达单shard
	if n := plannedShardCount(bson.D{
		{Key: "AccountId", Value: accountId},
		{Key: "_id", Value: playerId},
	}); n != 1 {
		t.Fatalf("filter{_id,AccountId} should target 1 shard, got %v", n)
	}
	// 带{AccountId,RegionId}(FindPlayerIdByAccountId的filter)→直达单shard
	if n := plannedShardCount(bson.D{
		{Key: "AccountId", Value: accountId},
		{Key: "RegionId", Value: 1},
	}); n != 1 {
		t.Fatalf("filter{AccountId,RegionId} should target 1 shard, got %v", n)
	}
	// 只带{_id}(FindEntityById的filter)→广播所有shard(未附加分片键的退化行为)
	if n := plannedShardCount(bson.D{
		{Key: "_id", Value: playerId},
	}); n != shardCount {
		t.Fatalf("filter{_id} should scatter-gather %v shards, got %v", shardCount, n)
	}

	// 顺带验证WithShardKey方法在分片集群上的功能正确性
	shardDb, ok := playerDb.(gentity.ShardKeyEntityDb)
	if !ok {
		t.Fatal("MongoCollectionPlayer should implement ShardKeyEntityDb")
	}
	var data shardTestPlayerData
	found, err := shardDb.FindEntityByIdWithShardKey(accountId, playerId, &data)
	if err != nil || !found || data.Id != playerId || data.Account != accountId {
		t.Fatalf("FindEntityByIdWithShardKey: found=%v err=%v data=%+v", found, err, data)
	}

	// 清理
	if err := client.Database(dbName).Drop(ctx); err != nil {
		t.Logf("drop database: %v", err)
	}
}
