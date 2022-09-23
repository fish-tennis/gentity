package gentity

import (
	"context"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/go-redis/redis/v8"
	"sync"
)

//type DistributedEntity interface {
//	RoutineEntity
//}

//type BaseDistributedEntity struct {
//	BaseRoutineEntity
//}

// 分布式实体管理类
type DistributedEntityMgr struct {
	// 分布式锁key
	distributedLockName string
	// 已加载实体
	entityMap     map[int64]RoutineEntity
	entityMapLock sync.RWMutex
	// GetEntity()==nil时,去加载实体数据
	loadEntityWhenGetNil bool
	// 数据库接口
	entityDb EntityDb
	// 缓存接口
	cache redis.Cmdable
	// 服务器列表接口
	serverList ServerList
	// 协程回调接口
	routineArgs *RoutineEntityRoutineArgs

	// 根据entityId路由到目标服务器
	// 返回值:服务器id
	routerFunc func(entityId int64) int32
	// 实体结构接口
	entityDataCreator func(entityId int64) interface{}
	// 根据数据创建实体接口
	entityCreator func(entityData interface{}) RoutineEntity
	// 消息转换成RoutineEntity的逻辑消息
	packetToRoutineMessage func(from Entity, packet gnet.Packet, to RoutineEntity) interface{}
	// 消息转换成路由消息
	packetToRemotePacket func(from Entity, packet gnet.Packet, toEntityId int64) gnet.Packet
	// 路由消息转换成RoutineEntity的逻辑消息
	remotePacketToRoutineMessage func(packet gnet.Packet, toEntityId int64) interface{}
}

func NewDistributedEntityMgr(distributedLockName string,
	entityDb EntityDb,
	cache redis.Cmdable,
	serverList ServerList,
	routineArgs *RoutineEntityRoutineArgs) *DistributedEntityMgr {
	return &DistributedEntityMgr{
		distributedLockName: distributedLockName,
		entityMap:           make(map[int64]RoutineEntity),
		entityDb:            entityDb,
		cache:               cache,
		serverList:          serverList,
		routineArgs:         routineArgs,
	}
}

// GetEntity()==nil时,去加载实体数据
func (this *DistributedEntityMgr) SetLoadEntityWhenGetNil(loadEntityWhenGetNil bool) {
	this.loadEntityWhenGetNil = loadEntityWhenGetNil
}

// 设置路由接口
func (this *DistributedEntityMgr) SetRouter(routerFunc func(entityId int64) int32) {
	this.routerFunc = routerFunc
}

// 设置实体创建接口
func (this *DistributedEntityMgr) SetCreator(entityDataCreator func(entityId int64) interface{},
	entityCreator func(entityData interface{}) RoutineEntity) {
	this.entityDataCreator = entityDataCreator
	this.entityCreator = entityCreator
}

// 设置消息转换接口
func (this *DistributedEntityMgr) SetPacketConvertor(
	packetToRoutineMessage func(from Entity, packet gnet.Packet, to RoutineEntity) interface{},
	packetToRemotePacket func(from Entity, packet gnet.Packet, toEntityId int64) gnet.Packet,
	remotePacketToRoutineMessage func(packet gnet.Packet, toEntityId int64) interface{}) {
	this.packetToRoutineMessage = packetToRoutineMessage
	this.packetToRemotePacket = packetToRemotePacket
	this.remotePacketToRoutineMessage = remotePacketToRoutineMessage
}

// 数据库接口
func (this *DistributedEntityMgr) GetEntityDb() EntityDb {
	return this.entityDb
}

// 获取已加载的分布式实体
func (this *DistributedEntityMgr) GetEntity(entityId int64) RoutineEntity {
	this.entityMapLock.Lock()
	defer this.entityMapLock.Unlock()
	return this.entityMap[entityId]
}

// 加载分布式实体
// 加载成功后,开启独立协程
func (this *DistributedEntityMgr) LoadEntity(entityId int64, entityData interface{}) RoutineEntity {
	// 到数据库加载数据
	exist, err := this.entityDb.FindEntityById(entityId, entityData)
	if err != nil {
		GetLogger().Debug("LoadEntity err:%v entityId:%v", err, entityId)
		return nil
	}
	if !exist {
		return nil
	}
	// 加载的数据生成实体对象
	newEntity := this.entityCreator(entityData)
	if newEntity == nil {
		GetLogger().Debug("LoadEntity newEntity==nil entityId:%v", entityId)
		return nil
	}
	this.entityMapLock.Lock()
	defer this.entityMapLock.Unlock()
	if existGuild, ok := this.entityMap[entityId]; ok {
		return existGuild
	}
	routineArgs := this.routineArgs
	if !newEntity.RunProcessRoutine(newEntity, &RoutineEntityRoutineArgs{
		InitFunc: func(routineEntity RoutineEntity) bool {
			if routineArgs.InitFunc != nil && !routineArgs.InitFunc(routineEntity) {
				return false
			}
			// 如果分布式锁Lock失败,则取消协程
			if !this.DistributeLock(routineEntity.GetId()) {
				return false
			}
			return true
		},
		EndFunc: func(routineEntity RoutineEntity) {
			if routineArgs.EndFunc != nil {
				routineArgs.EndFunc(routineEntity)
			}
			// 协程结束的时候,分布式锁UnLock
			this.DistributeUnlock(routineEntity.GetId())
			this.entityMapLock.Lock()
			defer this.entityMapLock.Unlock()
			delete(this.entityMap, routineEntity.GetId())
		},
		ProcessMessageFunc:    routineArgs.ProcessMessageFunc,
		AfterTimerExecuteFunc: routineArgs.AfterTimerExecuteFunc,
	}) {
		return nil
	}
	// 协程开启成功 才加入map
	this.entityMap[entityId] = newEntity
	return newEntity
}

// 分布式锁Lock
// redis实现的分布式锁,保证同一个实体的逻辑处理协程只会在一个服务器上
func (this *DistributedEntityMgr) DistributeLock(entityId int64) bool {
	// redis实现的分布式锁,保证同一个实体的逻辑处理协程只会在一个服务器上
	// 锁的是实体id和服务器id的对应关系
	lockOK, err := this.cache.HSetNX(context.Background(), this.distributedLockName, util.Itoa(entityId), GetApplication().GetId()).Result()
	if IsRedisError(err) {
		GetLogger().Error("%v.%v DistributeLock err:%v", this.distributedLockName, entityId, err.Error())
		return false
	}
	if !lockOK {
		GetLogger().Error("%v.%v DistributeLock failed", this.distributedLockName, entityId)
		return false
	}
	GetLogger().Debug("DistributeLock %v.%v", this.distributedLockName, entityId)
	return true
}

// 分布式锁UnLock
func (this *DistributedEntityMgr) DistributeUnlock(entityId int64) {
	this.cache.HDel(context.Background(), this.distributedLockName, util.Itoa(entityId))
	GetLogger().Debug("DistributeUnlock %v.%v", this.distributedLockName, entityId)
}

// 删除跟本服关联的分布式锁
func (this *DistributedEntityMgr) DeleteDistributeLocks() {
	kv, err := this.cache.HGetAll(context.Background(), this.distributedLockName).Result()
	if IsRedisError(err) {
		GetLogger().Error("DeleteDistributeLocks  %v err:%v", this.distributedLockName, err.Error())
		return
	}
	for entityIdStr, serverIdStr := range kv {
		if util.Atoi(serverIdStr) == int(GetApplication().GetId()) {
			this.cache.HDel(context.Background(), this.distributedLockName, entityIdStr)
			GetLogger().Debug("DeleteDistributeLocks %v.%v", this.distributedLockName, entityIdStr)
		}
	}
}

// 重新平衡
// 通知已不属于本服务器管理的实体关闭协程
func (this *DistributedEntityMgr) ReBalance() {
	this.entityMapLock.RLock()
	defer this.entityMapLock.RUnlock()
	for _, entity := range this.entityMap {
		if this.routerFunc(entity.GetId()) != GetApplication().GetId() {
			// 通知已不属于本服务器管理的实体关闭协程
			entity.Stop()
			GetLogger().Debug("distributedEntity stop %v", entity.GetId())
		}
	}
}

// 关闭所有实体协程
func (this *DistributedEntityMgr) StopAll() {
	this.entityMapLock.RLock()
	defer this.entityMapLock.RUnlock()
	for _, entity := range this.entityMap {
		// 通知已不属于本服务器管理的实体关闭协程
		entity.Stop()
		GetLogger().Debug("distributedEntity stop %v", entity.GetId())
	}
}

// 遍历
func (this *DistributedEntityMgr) Range(f func(entity RoutineEntity) bool) {
	this.entityMapLock.RLock()
	defer this.entityMapLock.RUnlock()
	for _, entity := range this.entityMap {
		if !f(entity) {
			return
		}
	}
}

// 路由消息
// 如果目标实体在本服务器上,则调用RoutineEntity.PushMessage
// 如果目标实体不在本服务器上,则根据路由规则查找其所在服务器,并封装路由消息发给目标服务器
func (this *DistributedEntityMgr) RoutePacket(from Entity, toEntityId int64, packet gnet.Packet) bool {
	routeServerId := this.routerFunc(toEntityId)
	if routeServerId == 0 {
		GetLogger().Debug("RoutePacket routeServerId==0 entityId:%v %v", toEntityId, packet)
		return false
	}
	// 目标实体在本服务器上
	if routeServerId == GetApplication().GetId() {
		toEntity := this.GetEntity(toEntityId)
		if toEntity == nil {
			if this.loadEntityWhenGetNil {
				entityData := this.entityDataCreator(toEntityId)
				toEntity = this.LoadEntity(toEntityId, entityData)
			}
			if toEntity == nil {
				GetLogger().Debug("RoutePacket entity==nil entityId:%v %v", toEntityId, packet)
				return false
			}
		}
		toEntity.PushMessage(this.packetToRoutineMessage(from, packet, toEntity))
	} else {
		routePacket := this.packetToRemotePacket(from, packet, toEntityId)
		this.serverList.Send(routeServerId, routePacket.Command(), routePacket.Message())
		GetLogger().Debug("RoutePacket routeServerId:%v entityId:%v %v", routeServerId, toEntityId, packet)
	}
	return false
}

// 处理另一个服务器转发过来的路由消息
// 解析出实际的消息,放入目标实体的消息队列中
func (this *DistributedEntityMgr) ParseRoutePacket(toEntityId int64, packet gnet.Packet) {
	// 再验证一次是否属于本服务器管理
	if this.routerFunc(toEntityId) != GetApplication().GetId() {
		GetLogger().Debug("route err entityId:%v %v", toEntityId, packet)
		return
	}
	toEntity := this.GetEntity(toEntityId)
	if toEntity == nil {
		if this.loadEntityWhenGetNil {
			entityData := this.entityDataCreator(toEntityId)
			toEntity = this.LoadEntity(toEntityId, entityData)
		}
		if toEntity == nil {
			GetLogger().Debug("OnRecvRoutePacket entity==nil entityId:%v %v", toEntityId, packet)
			return
		}
	}
	routineMessage := this.remotePacketToRoutineMessage(packet, toEntityId)
	if routineMessage == nil {
		GetLogger().Debug("ParseRoutePacket convert err entityId:%v %v", toEntityId, packet)
		return
	}
	toEntity.PushMessage(routineMessage)
}
