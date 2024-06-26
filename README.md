# gentity
游戏服务器中的实体对象的序列化,数据读写,缓存等接口,通过配置即可实现对象的数据保存加载,缓存更新,极大的简化编码

基于gentity,游戏服务器框架可以更快的构建

网络库使用[gnet](https://github.com/fish-tennis/gnet)

![gentity](https://github.com/fish-tennis/doc/blob/master/imgs/gentity/gentity.png)

## Entity-Component
Entity-Component模式是类似Unity的GameObject-Component的实体组件模式,便于组件解耦

比如游戏服务器中的玩家对象就属于Entity,玩家的任务模块就是一个Component

- 组件模块注册
- 组件消息回调接口注册
- 组件事件分发
- 组件事件响应接口注册

## 实体数据
实体数据的加载和保存是游戏服务器最基础的功能,gentity利用go的struct tag,大大简化了实体数据加载和保存的接口

gentity抽象出了实体的数据库接口EntityDb和实体的缓存接口KvCache

gentity内置了EntityDb的mongodb实现,和KvCache的redis实现

## struct tag
使用go的struct tag,设置对象组件的字段,框架接口会自动对这些字段进行数据库读取保存和缓存更新,极大的简化了业务代码对数据库和缓存的操作

设置组件保存数据
```go
// entity的一个组件
type Money struct {
	// 该字段必须导出(首字母大写)
	// 使用struct tag来标记该字段需要存数据库,可以设置存储字段名(proto格式存mongo时,使用全小写格式)
	Data *pb.Money `db:"money"`
}
//entity.SaveDb()会自动把Money.Data保存到mongo,保存时会自动进行proto序列化
//entity.SaveCache()会自动把Money.Data缓存到redis,保存时会自动进行proto序列化
```

支持明文方式保存数据
```go
// 玩家基础信息组件
type BaseInfo struct {
	// plain表示明文存储,在保存到mongodb时,不会进行proto序列化
	Data *pb.BaseInfo `db:"baseinfo;plain"`
}
```

支持组合模式
```go
// 任务组件
type Quest struct {
	// 保存数据的子模块:已完成的任务
	Finished *FinishedQuests `child:"finished"`
	// 保存数据的子模块:当前任务列表
	Quests *CurQuests `child:"quests"`
}
// 已完成的任务
type FinishedQuests struct {
    BaseDirtyMark
    // 明文保存
    Finished []int32 `db:"finished;plain"`
}
// 当前任务列表
type CurQuests struct {
    BaseMapDirtyMark
    // protobuf方式保存
    Quests map[int32]*pb.QuestData `db:"quests"`
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
分布式实体DistributedEntity在RoutineEntity的基础上增加了数据库加载接口,分布式锁接口,消息路由接口

示例:[gserver](https://github.com/fish-tennis/gserver) 里的公会对象Guild

![distributed entity](https://github.com/fish-tennis/doc/blob/master/imgs/gentity/distributedentity.png)

## 服务器自动组网
服务器注册,发现,组网

## 项目演示
分布式游戏服务器框架[gserver](https://github.com/fish-tennis/gserver)

## 讨论
QQ群: 764912827