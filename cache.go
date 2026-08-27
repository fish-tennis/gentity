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

// 支持Lua脚本原子操作的缓存接口(可选实现)
// 多条缓存命令如果分多次网络请求执行,命令之间存在中间状态:
//   - 并发读会读到中间状态(如:map已Del但还没写入新数据)
//   - 部分成功后进程崩溃,会留下半成品数据
//   - 调用方基于旧快照做条件操作,会误删其他持有者已更新的数据
//
// Redis实现通过Lua脚本在服务器端原子执行,消除上述问题
// 使用方式:通过接口断言判断缓存实现是否支持,不支持则回退到分步执行
//
//	if scriptCache, ok := kvCache.(ScriptKvCache); ok { ... } else { 分步回退 }
//
// NOTE:Redis Cluster不支持跨节点的Lua脚本(要求所有key在同一个slot),
// 本接口的所有方法均只操作单个key,天然满足集群约束
type ScriptKvCache interface {
	// 原子替换map缓存:等价于 Del(key) + SetMap(key, m) 在服务器端原子执行
	// 用于map整体覆盖场景,避免Del和写入之间出现缓存丢失或读到中间状态
	// m为空map时等价于Del
	AtomicReplaceMap(key string, m interface{}) error

	// 原子增量更新map缓存:等价于 SetMap(key, setMap) + HDel(key, delFields...) 在服务器端原子执行
	// 分步执行时若set成功del失败(或中途崩溃),调用方重置脏标记后不再重试,
	// 将导致内存与缓存永久不一致,最终把错误数据落库;原子执行则不会出现半成品状态
	AtomicUpdateMap(key string, setMap interface{}, delFields []string) error

	// 原子条件写入hash field:仅当field不存在或field的值等于value时才写入
	// field值等于value时也返回成功,用于可重入的分布式锁:
	// 服务器重启后重新获取自己上次崩溃残留的锁
	// 返回是否写入成功
	HSetIfAbsentOrEqual(key, field string, value interface{}) (bool, error)

	// 原子条件删除hash field:仅当field的值等于expectValue时才删除
	// 防止基于旧状态的无条件删除误删其他持有者(如新持有者)已更新的数据
	// 返回是否删除成功
	HDelIfValueEqual(key, field string, expectValue interface{}) (bool, error)

	// 原子删除所有值等于value的hash field
	// 等价于 HGetAll + 条件HDel 的原子执行,消除快照与删除之间其他持有者重新写入后被误删的竞态
	// 返回删除的field数量
	HDelFieldsByValue(key string, value interface{}) (int64, error)
}
