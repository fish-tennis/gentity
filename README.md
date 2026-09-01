# gentity
实体对象的数据绑定(类似gorm),数据读写(序列化),缓存等接口,通过配置即可实现对象的数据保存加载,缓存更新,以简化业务编码,并使业务代码和数据库&缓存解耦

基于gentity,游戏服务器框架可以更快的构建

![gentity](https://github.com/fish-tennis/doc/blob/master/imgs/gentity/gentity.png)

## Entity-Component
Entity-Component模式是类似Unity的GameObject-Component的实体组件模式,便于组件解耦

比如游戏服务器中的玩家对象就属于Entity,玩家的任务模块就是一个Component

- 组件模块注册
- 组件消息回调接口注册
- 组件事件分发
- 组件事件响应接口注册

## 数据库和缓存
实体数据的加载和保存是游戏服务器最基础的功能,gentity利用go的struct tag,大大简化了实体数据加载和保存的接口

gentity抽象出了实体的数据库接口EntityDb和实体的缓存接口KvCache,并且可以自动检查增量更新,只把修改过的数据保存到数据库和缓存

gentity内置了EntityDb的mongodb实现,和KvCache的redis实现

## 数据绑定
类似gorm(go Object Relation Mapping)对SQL进行对象映射,gentity的数据绑定对组件进行数据库和缓存的映射

利用go的struct tag,设置对象组件的字段,框架接口会自动对这些字段进行数据库读取保存和缓存更新,极大的简化了业务代码对数据库和缓存的操作

设置组件保存数据
```go
// entity的一个组件
type Money struct {
    DataComponent
    // 该字段必须导出(首字母大写)
    // 使用struct tag来标记该字段需要存数据库
    Data *pb.Money `db:""`
}
```

支持明文方式保存数据
```go
// 玩家基础信息组件
type BaseInfo struct {
    DataComponent
    // plain表示明文存储,在保存到mongodb时,不会进行proto序列化
    Data *pb.BaseInfo `db:"plain"`
}
```

支持多个保存字段
```go
// 任务组件
type Quest struct {
    BaseComponent
    // 保存数据的子模块:已完成的任务 使用明文保存方式
    // wrapper of []int32
    Finished *gentity.SliceData[int32] `child:"Finished;plain"`
    // 保存数据的子模块:当前任务列表
    // wrapper of map[int32]*pb.QuestData
    Quests *gentity.MapData[int32, *pb.QuestData] `child:"Quests"`
}
```

## 消息回调
支持自动注册消息回调,事件响应
```go
// 客户端发给服务器的完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Quest) OnFinishQuestReq(reqCmd gnet.PacketCommand, req *pb.FinishQuestReq) {
	// logic code ...
}
```
```go
// 这种格式写的函数可以自动注册非客户端的消息回调
func (this *BaseInfo) HandlePlayerEntryGameOk(cmd gnet.PacketCommand, msg *pb.PlayerEntryGameOk) { 
	// logic code ...
}
```
```go
// 这种格式写的函数可以自动注册事件响应接口
// 当执行player.FireEvent(&EventPlayerEntryGame{})时,该响应接口会被调用
func (this *Quest) TriggerPlayerEntryGame(event *EventPlayerEntryGame) {
	// logic code ...
}
```

## 独立协程实体RoutineEntity
每个RoutineEntity分配一个独立的逻辑协程,在自己的独立协程中执行只涉及自身数据的代码,无需加锁

同时,RoutineEntity内置了一个协程安全的计时器

![routine entity](https://github.com/fish-tennis/doc/blob/master/imgs/gentity/routineentity.png)

示例:[gserver](https://github.com/fish-tennis/gserver) 里的玩家对象Player

## 分布式实体DistributedEntity
分布式实体DistributedEntity在RoutineEntity的基础上增加了数据库加载接口,分布式锁接口

示例:[gserver](https://github.com/fish-tennis/gserver) 里的公会对象Guild

![distributed entity](https://github.com/fish-tennis/doc/blob/master/imgs/gentity/distributedentity.png)

## 数据库分片(MongoDB Sharding)
分片是可选项:注册collection时通过`ShardKeyType`指定分片方式,`ShardKeyNone`表示不分片

```go
const (
	ShardKeyNone   // 不分片(默认)
	ShardKeyRange  // range分片
	ShardKeyHashed // hashed分片
)

mongoDb := gentity.NewMongoDb(uri, dbName)
// 注册时声明分片方式(必须在Connect()之前)
mongoDb.RegisterPlayerDb("player", gentity.ShardKeyHashed, "_id", "AccountId", "RegionId")
mongoDb.RegisterEntityDb("guild", gentity.ShardKeyNone, "_id")
// Connect()会为已注册的collection回填连接并创建唯一索引
mongoDb.Connect()
// 分片集群环境下执行enableSharding+shardCollection(单机mongo无此命令,属预期降级)
mongoDb.ShardDatabase(dbName)
```

### 分片键与uniqueId不同的collection
典型场景:player表分片键为`AccountId`,而entityKey是`_id`(playerId)

分片集群下,按entityKey的读写不含分片键时会被mongos广播到所有分片.gentity提供两个可选接口解决:

- collection侧:`SetShardKeyName`启用`ShardKeyEntityDb`,读写方法可附加分片键条件直达目标分片
- entity侧:实现`ShardKeyProvider`,存盘链路(`SaveEntityChangedDataToDb`)自动附加分片键条件

```go
// 注册时启用分片键附加条件(分片键列AccountId,与uniqueId _id不同)
playerDb := mongoDb.RegisterPlayerDb("player", gentity.ShardKeyHashed, "_id", "AccountId", "RegionId")
playerDb.(*gentity.MongoCollectionPlayer).SetShardKeyName("AccountId")

// entity实现ShardKeyProvider,提供分片键的值
func (this *Player) GetShardKeyValue() interface{} {
	return this.AccountId
}

// 此后玩家存盘(SaveEntityChangedDataToDb)自动附加分片键条件,直达分片;
// 显式读写也可以附加分片键条件:
if shardDb, ok := playerDb.(gentity.ShardKeyEntityDb); ok {
	shardDb.FindEntityByIdWithShardKey(accountId, playerId, &playerData)
	shardDb.SaveComponentWithShardKey(accountId, playerId, "Bag", bagData)
}
```

设计原则:未附加分片键时仅性能退化为广播,不影响正确性,因此可以渐进式启用;`InsertEntity`无需分片键变体(插入的文档本身携带分片键字段,mongos自动按其路由),按分片键列查询的接口(如`FindPlayerIdByAccountId`)天然直达

分片环境说明:分片集合的唯一索引必须以分片键为前缀,因此分片键不再是`_id`时,`_id`的唯一性由应用层保证(如全局自增id分配);角色名等与分片键无关的全局唯一约束无法再用唯一索引保证

相关示例与测试(gentity/examples/mongo_test.go):
- `TestShardKeyEntityDb`:分片键附加条件的读写语义(单机mongo可跑)
- `TestSaveEntityChangedDataToDbWithShardKey`:存盘链路自动附加分片键(单机mongo可跑)
- `TestShardKeyRoutingOnShardedCluster`:分片集群下explain验证直达/广播路由(需gserver的docker/mongo_sharded环境,未就绪时自动Skip)

## 项目演示
分布式游戏服务器框架[gserver](https://github.com/fish-tennis/gserver)

## 讨论
QQ群: 764912827
