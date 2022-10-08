package gentity

// Entity的数据库接口
type EntityDb interface {
	// 根据id查找数据
	FindEntityById(entityId int64, data interface{}) (bool, error)

	//// 根据名字查找数据
	//FindEntityByName(name string, data interface{}) (bool, error)

	// 新建Entity(insert)
	InsertEntity(entityId int64, entityData interface{}) (err error, isDuplicateKey bool)

	// 保存Entity数据(update entity by entityId)
	SaveEntity(entityId int64, entityData interface{}) error

	// 保存1个组件(update entity's component)
	SaveComponent(entityId int64, componentName string, componentData interface{}) error

	// 批量保存组件(update entity's components...)
	SaveComponents(entityId int64, components map[string]interface{}) error

	// 保存1个组件的一个字段(update entity's component.field)
	SaveComponentField(entityId int64, componentName string, fieldName string, fieldData interface{}) error

	// 删除1个组件的某些字段
	DeleteComponentField(entityId int64, componentName string, fieldName... string) error
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
