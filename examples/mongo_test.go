package examples

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/examples/pb"
	"github.com/fish-tennis/gentity/util"
	"sync"
	"testing"
)

// mongo实现的自增id方式
func TestIncrementId(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	kvDb := mongoDb.RegisterKvDb("kv", false, "k", "v")
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
	kvDb := mongoDb.RegisterKvDb("kv", false, "k", "v")
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
	playerDb := mongoDb.RegisterPlayerDb("player", true, "_id", "AccountId", "RegionId")
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
