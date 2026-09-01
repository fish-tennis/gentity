package gentity

// Entity的数据库接口
type EntityDb interface {
	// 根据id查找数据
	FindEntityById(entityKey interface{}, data interface{}) (bool, error)

	// 新建Entity(insert)
	InsertEntity(entityKey interface{}, entityData interface{}) (err error, isDuplicateKey bool)

	// 保存Entity数据(update entity by entityKey)
	SaveEntity(entityKey interface{}, entityData interface{}) error

	// 删除Entity数据(delete entity by entityKey)
	DeleteEntity(entityKey interface{}) error

	// 保存1个组件(update entity's component)
	SaveComponent(entityKey interface{}, componentName string, componentData interface{}) error

	// 批量保存组件(update entity's components...)
	SaveComponents(entityKey interface{}, components map[string]interface{}) error

	// 保存1个组件的一个字段(update entity's component.field)
	SaveComponentField(entityKey interface{}, componentName string, fieldName string, fieldData interface{}) error

	// 删除1个组件的某些字段
	DeleteComponentField(entityKey interface{}, componentName string, fieldName ...string) error
	// TODO:需要一个有容量限制的列表接口,用于邮件或者离线操作之类的接口
}

// 玩家数据接口
// Db接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
type PlayerDb interface {
	EntityDb

	// 根据账号id查找角色id
	// 适用于一个账号在一个区服只有一个玩家角色的游戏
	FindPlayerIdByAccountId(accountId int64, regionId int32) (int64, error)

	// MMORPG类型的游戏,可能一个账号在一个服有多个角色
	FindPlayerIdsByAccountId(accountId int64, regionId int32) ([]int64, error)

	// 根据账号id查找玩家数据
	// 适用于一个账号在一个区服只有一个玩家角色的游戏
	FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool, error)

	// 根据角色id查找账号id
	FindAccountIdByPlayerId(playerId int64) (int64, error)
}

// 分片键附加条件的EntityDb可选接口
// 适用场景:分片键与uniqueId不同的collection(如player表:分片键AccountId,uniqueId _id)
//
// 分片集群下,按entityKey的读写(如FindEntityById/SaveEntity)不含分片键时,
// 会被mongos广播到所有分片;附加分片键条件后可直达目标分片.
// 未附加分片键时仅性能退化(广播),不影响正确性,因此本接口是可选增强,
// 调用方通过类型断言使用:
//
//	if shardDb, ok := db.GetEntityDb(name).(ShardKeyEntityDb); ok && shardDb.ShardKeyName() != "" {
//	    shardDb.FindEntityByIdWithShardKey(accountId, playerId, data)
//	}
type ShardKeyEntityDb interface {
	EntityDb

	// 分片键列名,空表示该collection未启用分片键附加条件
	ShardKeyName() string

	// 根据id+分片键查找数据(直达分片)
	FindEntityByIdWithShardKey(shardKeyValue, entityKey interface{}, data interface{}) (bool, error)

	// 保存Entity数据(update entity by entityKey+shardKey,直达分片)
	SaveEntityWithShardKey(shardKeyValue, entityKey interface{}, entityData interface{}) error

	// 删除Entity数据(delete entity by entityKey+shardKey,直达分片)
	DeleteEntityWithShardKey(shardKeyValue, entityKey interface{}) error

	// 保存1个组件(update entity's component,直达分片)
	SaveComponentWithShardKey(shardKeyValue, entityKey interface{}, componentName string, componentData interface{}) error

	// 批量保存组件(update entity's components...,直达分片)
	SaveComponentsWithShardKey(shardKeyValue, entityKey interface{}, components map[string]interface{}) error

	// 保存1个组件的一个字段(update entity's component.field,直达分片)
	SaveComponentFieldWithShardKey(shardKeyValue, entityKey interface{}, componentName string, fieldName string, fieldData interface{}) error

	// 删除1个组件的某些字段(直达分片)
	DeleteComponentFieldWithShardKey(shardKeyValue, entityKey interface{}, componentName string, fieldName ...string) error

	// NOTE:InsertEntity无需分片键变体:插入的文档本身携带分片键字段,mongos自动按其路由;
	// 按分片键列查询的接口(如PlayerDb.FindPlayerIdByAccountId)天然含分片键,同样直达
}

// Kv数据接口
// 游戏应用里,除了账号数据和玩家数据之外,其他以Key-Value存储的数据
// KvDb接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
type KvDb interface {
	Find(key interface{}) (interface{}, error)

	FindAndDecode(key interface{}, decodeData interface{}) error

	Insert(key interface{}, value interface{}) (err error, isDuplicateKey bool)

	Update(key interface{}, value interface{}, upsert bool) error

	Inc(key interface{}, value interface{}, upsert bool) (interface{}, error)

	Delete(key interface{}) error
}

// 数据表管理接口
type DbMgr interface {
	GetEntityDb(name string) EntityDb
	GetKvDb(name string) KvDb
}
