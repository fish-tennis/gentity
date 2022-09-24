package gentity

import (
	"google.golang.org/protobuf/proto"
	"time"
)

// 常用的kv缓存接口
type KvCache interface {
	// redis Get
	Get(key string) (string, error)

	// redis Set
	// value如果是proto.Message,会先进行序列化
	Set(key string, value interface{}, expiration time.Duration) error

	// redis SetNX
	// value如果是proto.Message,会先进行序列化
	SetNX(key string, value interface{}, expiration time.Duration) (bool, error)

	// redis Del
	Del(key ...string) (int64,error)

	// redis Type
	Type(key string) (string, error)

	// 缓存数据加载到map
	// m必须是一个类型明确有效的map,且key类型只能是int或string,value类型只能是int或string或proto.Message
	//
	// example:
	//   testDataMap := make(map[int64]*pb.TestData)
	//   HGetAll("myhash", testDataMap)
	GetMap(key string, m interface{}) error

	// map数据缓存
	// m必须是一个类型明确有效的map,且key类型只能是int或string,value类型只能是int或string或proto.Message
	// NOTE:批量写入数据,并不会删除之前缓存的数据
	//
	// example:
	//   - SetMap("myhash", map[int64]*pb.TestData{1:&pb.TestData{},2:&pb.TestData{}})
	//   - SetMap("myhash", map[string]interface{}{"key1": "value1", "key2": "value2"})
	SetMap(key string, m interface{}) error

	// redis HGetAll
	HGetAll(key string) (map[string]string,error)

	// redis HSet
	// HSet accepts values in following formats:
	//   - HSet("myhash", "key1", "value1", "key2", "value2")
	//   - HSet("myhash", []string{"key1", "value1", "key2", "value2"})
	//   - HSet("myhash", map[string]interface{}{"key1": "value1", "key2": "value2"})
	HSet(key string, values ...interface{}) (int64,error)

	// redis HSetNX
	HSetNX(key, field string, value interface{}) (bool,error)

	// 删除map的项
	HDel(key string, fields ...string) (int64,error)

	// 缓存数据加载到proto.Message
	GetProto(key string, value proto.Message) error
}
