package gentity

import (
	"sync"

	"github.com/fish-tennis/gentity/util"
)

// DistributedEntity的回调接口
type DistributedEntityHelper interface {
	// 创建实体
	CreateEntity(entityData interface{}) RoutineEntity
	// 根据entityId路由到目标服务器
	// 返回值:服务器id
	RouteServerId(entityId int64) int32
}

// 分布式实体管理类
type DistributedEntityMgr struct {
	// 分布式锁key
	distributedLockName string
	// 已加载实体
	entityMap     map[int64]RoutineEntity
	entityMapLock sync.RWMutex
	// 数据库接口
	entityDb EntityDb
	// 缓存接口
	cache KvCache
	// 协程回调接口
	routineArgs *RoutineEntityRoutineArgs
	// DistributedEntity的回调接口
	distributedEntityHelper DistributedEntityHelper
}

func NewDistributedEntityMgr(distributedLockName string,
	entityDb EntityDb,
	cache KvCache,
	routineArgs *RoutineEntityRoutineArgs,
	distributedEntityHelper DistributedEntityHelper) *DistributedEntityMgr {
	return &DistributedEntityMgr{
		distributedLockName:     distributedLockName,
		entityMap:               make(map[int64]RoutineEntity),
		entityDb:                entityDb,
		cache:                   cache,
		routineArgs:             routineArgs,
		distributedEntityHelper: distributedEntityHelper,
	}
}

// 数据库接口
func (this *DistributedEntityMgr) GetEntityDb() EntityDb {
	return this.entityDb
}

// 获取已加载的分布式实体
func (this *DistributedEntityMgr) GetEntity(entityId int64) RoutineEntity {
	this.entityMapLock.RLock()
	defer this.entityMapLock.RUnlock()
	return this.entityMap[entityId]
}

// 加载分布式实体
// 加载成功后,开启独立协程
func (this *DistributedEntityMgr) LoadEntity(entityId int64, entityData interface{}) RoutineEntity {
	// 快速预检查(读锁),避免重复到数据库查询
	if e := this.GetEntity(entityId); e != nil {
		return e
	}
	// 到数据库加载数据
	exist, err := this.entityDb.FindEntityById(entityId, entityData)
	if err != nil {
		glog.Debug("LoadEntity", "err", err, "entityId", entityId)
		return nil
	}
	if !exist {
		return nil
	}
	// 加载的数据生成实体对象
	newEntity := this.distributedEntityHelper.CreateEntity(entityData)
	if newEntity == nil {
		glog.Debug("LoadEntity newEntity==nil", "entityId", entityId)
		return nil
	}
	// 先在写锁外获取分布式锁,避免持锁期间执行Redis网络IO导致其它实体操作被串行化阻塞
	if !this.DistributeLock(entityId) {
		return nil
	}
	// 加写锁,双重检查,避免并发加载同一个entityId
	this.entityMapLock.Lock()
	if existEntity, ok := this.entityMap[entityId]; ok {
		this.entityMapLock.Unlock()
		// 已经有其他协程加载成功了,释放刚才获取的锁
		this.DistributeUnlock(entityId)
		return existEntity
	}
	routineArgs := this.routineArgs
	// 分布式锁已经在上面获取,InitFunc不再调用DistributeLock
	startOK := newEntity.RunProcessRoutine(newEntity, &RoutineEntityRoutineArgs{
		InitFunc: func(routineEntity RoutineEntity) bool {
			if routineArgs.InitFunc != nil && !routineArgs.InitFunc(routineEntity) {
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
			delete(this.entityMap, routineEntity.GetId())
			this.entityMapLock.Unlock()
		},
		ProcessMessageFunc:    routineArgs.ProcessMessageFunc,
		AfterTimerExecuteFunc: routineArgs.AfterTimerExecuteFunc,
	})
	if !startOK {
		// 协程启动失败,释放锁
		this.entityMapLock.Unlock()
		this.DistributeUnlock(entityId)
		return nil
	}
	// 协程开启成功 才加入map
	this.entityMap[entityId] = newEntity
	this.entityMapLock.Unlock()
	return newEntity
}

// 分布式锁Lock
// redis实现的分布式锁,保证同一个实体的逻辑处理协程只会在一个服务器上
func (this *DistributedEntityMgr) DistributeLock(entityId int64) bool {
	// 锁的是实体id和服务器id的对应关系
	field := util.Itoa(entityId)
	owner := util.Itoa(GetApplication().GetId())
	if scriptCache, ok := this.cache.(ScriptKvCache); ok {
		// Lua脚本原子执行:field不存在或仍是本服持有时写入
		// HSetNX无法识别本服崩溃后的残留锁,会导致重启后无法重新加载自己的实体
		lockOK, err := scriptCache.HSetIfAbsentOrEqual(this.distributedLockName, field, owner)
		if IsRedisError(err) {
			glog.Error("DistributeLock", "lockName", this.distributedLockName, "entityId", entityId, "err", err)
			return false
		}
		if !lockOK {
			glog.Error("DistributeLock failed", "lockName", this.distributedLockName, "entityId", entityId)
			return false
		}
		glog.Debug("DistributeLock", "lockName", this.distributedLockName, "entityId", entityId)
		return true
	}
	// 回退:不支持脚本的缓存系统,使用HSetNX
	// (无法识别本服崩溃残留的锁,需依赖DeleteDistributeLocks在启动时清理)
	lockOK, err := this.cache.HSetNX(this.distributedLockName, util.Itoa(entityId), GetApplication().GetId())
	if IsRedisError(err) {
		glog.Error("DistributeLock", "lockName", this.distributedLockName, "entityId", entityId, "err", err)
		return false
	}
	if !lockOK {
		glog.Error("DistributeLock failed", "lockName", this.distributedLockName, "entityId", entityId)
		return false
	}
	glog.Debug("DistributeLock", "lockName", this.distributedLockName, "entityId", entityId)
	return true
}

// 分布式锁UnLock
func (this *DistributedEntityMgr) DistributeUnlock(entityId int64) {
	field := util.Itoa(entityId)
	owner := util.Itoa(GetApplication().GetId())
	if scriptCache, ok := this.cache.(ScriptKvCache); ok {
		// Lua脚本原子执行:仅当锁仍由本服持有时才删除
		// 无条件HDEL存在误删风险:本服的解锁请求因网络延迟晚到时,
		// field可能已被其他服务器重新加锁,无条件删除会破坏对方的锁,
		// 导致同一个实体在两台服务器上同时存在
		_, err := scriptCache.HDelIfValueEqual(this.distributedLockName, field, owner)
		if IsRedisError(err) {
			glog.Error("DistributeUnlock", "lockName", this.distributedLockName, "entityId", entityId, "err", err)
		}
		glog.Debug("DistributeUnlock", "lockName", this.distributedLockName, "entityId", entityId)
		return
	}
	this.cache.HDel(this.distributedLockName, util.Itoa(entityId))
	glog.Debug("DistributeUnlock", "lockName", this.distributedLockName, "entityId", entityId)
}

// 删除跟本服关联的分布式锁
func (this *DistributedEntityMgr) DeleteDistributeLocks() {
	owner := util.Itoa(GetApplication().GetId())
	if scriptCache, ok := this.cache.(ScriptKvCache); ok {
		// Lua脚本原子执行 HGetAll+条件HDel
		// 分步执行时,HGetAll读到的快照与HDel之间,本服可能已释放某field且被其他服务器重新加锁,
		// 无条件HDel会误删其他服务器的新锁,导致同一个实体在两台服务器上同时存在
		deleted, err := scriptCache.HDelFieldsByValue(this.distributedLockName, owner)
		if IsRedisError(err) {
			glog.Error("DeleteDistributeLocks", "lockName", this.distributedLockName, "err", err)
			return
		}
		glog.Debug("DeleteDistributeLocks", "lockName", this.distributedLockName, "deleted", deleted)
		return
	}
	// 回退:HGetAll快照+逐个HDel(存在快照竞态,仅用于不支持脚本的缓存系统)
	kv, err := this.cache.HGetAll(this.distributedLockName)
	if IsRedisError(err) {
		glog.Error("DeleteDistributeLocks", "lockName", this.distributedLockName, "err", err)
		return
	}
	for entityIdStr, serverIdStr := range kv {
		if util.Atoi(serverIdStr) == int(GetApplication().GetId()) {
			this.cache.HDel(this.distributedLockName, entityIdStr)
			glog.Debug("DeleteDistributeLocks", "lockName", this.distributedLockName, "entityId", entityIdStr)
		}
	}
}

// 重新平衡
// 通知已不属于本服务器管理的实体关闭协程
func (this *DistributedEntityMgr) ReBalance() {
	this.entityMapLock.RLock()
	defer this.entityMapLock.RUnlock()
	for _, entity := range this.entityMap {
		if this.distributedEntityHelper.RouteServerId(entity.GetId()) != GetApplication().GetId() {
			// 通知已不属于本服务器管理的实体关闭协程
			entity.Stop()
			glog.Debug("distributedEntity stop", "entityId", entity.GetId())
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
		glog.Debug("distributedEntity stop", "entityId", entity.GetId())
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
