package gentity

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ PlayerDb = (*MongoCollectionPlayer)(nil)
var _ EntityDb = (*MongoCollection)(nil)

type Sharding interface {
	Shard() error
}

// 分片key类型
type ShardKeyType int

const (
	// 不分片
	ShardKeyNone ShardKeyType = iota
	// range分片key
	ShardKeyRange
	// hashed分片key
	ShardKeyHashed
)

// db.EntityDb的mongo实现
type MongoCollection struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database
	shardKeyType  ShardKeyType

	// 表名
	collectionName string
	// 唯一id
	uniqueId string
	// 分片键列名(可选)
	// 非空时Shard()以该列作为分片键,并启用ShardKeyEntityDb的分片键附加条件方法;
	// 典型场景:player表分片键AccountId与uniqueId _id不同,
	// 按playerId的读写附加AccountId条件后可直达分片(见db.go的ShardKeyEntityDb)
	shardKeyName string
}

// SetShardKeyName 设置分片键列名,启用ShardKeyEntityDb的分片键附加条件方法
// NOTE:须在Connect()/ShardDatabase()之前调用:Shard()在ShardDatabase时读取该列作为分片键
func (this *MongoCollection) SetShardKeyName(shardKeyName string) {
	this.shardKeyName = shardKeyName
	glog.Info("SetShardKeyName", "collection", this.collectionName, "shardKeyName", shardKeyName)
}

func (this *MongoCollection) ShardKeyName() string {
	return this.shardKeyName
}

func (this *MongoCollection) GetCollection() *mongo.Collection {
	return this.mongoDatabase.Collection(this.collectionName)
}

func (this *MongoCollection) CreateIndex(key string, unique bool) {
	col := this.mongoDatabase.Collection(this.collectionName)
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: key, Value: 1},
		},
		Options: options.Index().SetUnique(unique),
	}
	indexName, indexErr := col.Indexes().CreateOne(context.Background(), indexModel)
	if indexErr != nil {
		glog.Error("create index", "collection", this.collectionName, "index", indexName, "err", indexErr)
	} else {
		glog.Info("index", "collection", this.collectionName, "indexName", indexName)
	}
}

// 设置分片key
// ShardKeyNone表示不分片,直接跳过
func (this *MongoCollection) Shard() error {
	if this.shardKeyType == ShardKeyNone {
		glog.Info("Shard skip", "collection", this.collectionName)
		return nil
	}
	collectionFullName := fmt.Sprintf("%v.%v", this.mongoDatabase.Name(), this.collectionName)
	// 分片键列优先使用shardKeyName,未设置时退化为uniqueId(两者相同的常规场景)
	shardKeyName := this.shardKeyName
	if shardKeyName == "" {
		shardKeyName = this.uniqueId
	}
	key := bson.E{Key: shardKeyName, Value: 1}
	if this.shardKeyType == ShardKeyHashed {
		key.Value = "hashed"
	}
	err := this.mongoClient.Database("admin").RunCommand(context.Background(), bson.D{
		{Key: "shardCollection", Value: collectionFullName},
		{Key: "key", Value: bson.D{key}},
	}).Err()
	if err != nil {
		glog.Error("Shard", "collection", collectionFullName, "err", err)
	} else {
		glog.Info("Shard", "collection", collectionFullName, "shardKeyType", this.shardKeyType)
	}
	return err
}

// 根据id查找数据
func (this *MongoCollection) FindEntityById(entityKey interface{}, data interface{}) (bool, error) {
	if len(this.uniqueId) == 0 {
		return false, ErrNoUniqueColumn
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *MongoCollection) InsertEntity(entityKey interface{}, entityData interface{}) (err error, isDuplicateKey bool) {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err = col.InsertOne(context.Background(), entityData)
	if err != nil {
		isDuplicateKey = IsDuplicateKeyError(err)
	}
	return
}

func (this *MongoCollection) SaveEntity(entityKey interface{}, entityData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.UpdateOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}}, entityData)
	return err
}

func (this *MongoCollection) DeleteEntity(entityKey interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.DeleteOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}})
	return err
}

func (this *MongoCollection) SaveComponent(entityKey interface{}, componentName string, componentData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}},
		bson.D{{Key: "$set", Value: bson.D{{Key: componentName, Value: componentData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoCollection) SaveComponents(entityKey interface{}, components map[string]interface{}) error {
	if len(components) == 0 {
		return nil
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	// filter含唯一键(Connect时建了唯一索引),最多匹配1个文档,UpdateOne语义准确
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}},
		bson.D{{Key: "$set", Value: components}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoCollection) SaveComponentField(entityKey interface{}, componentName string, fieldName string, fieldData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	// NOTE:如果player.ComponentName == null
	// 直接更新player.ComponentName.fieldName会报错: Cannot create field 'fieldName' in element
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}},
		bson.D{{Key: "$set", Value: bson.D{{Key: componentName + "." + fieldName, Value: fieldData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// 删除1个组件的某些字段
func (this *MongoCollection) DeleteComponentField(entityKey interface{}, componentName string, fieldName ...string) error {
	if len(fieldName) == 0 {
		return nil
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	fieldNames := bson.D{}
	for _, name := range fieldName {
		fieldNames = append(fieldNames, bson.E{Key: componentName + "." + name})
	}
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{Key: this.uniqueId, Value: entityKey}},
		bson.D{{Key: "$unset", Value: fieldNames}})
	if updateErr != nil {
		return updateErr
	}
	//glog.Debug("DeleteComponentField", "result", result)
	return nil
}

// ==================== ShardKeyEntityDb实现 ====================
// 分片键与uniqueId不同的collection(如player表:分片键AccountId,uniqueId _id)使用,
// filter附加分片键条件后mongos可直达目标分片,避免广播;详见db.go的ShardKeyEntityDb

// shardFilter 构造"{分片键列: shardKeyValue, uniqueId: entityKey}"查询条件
func (this *MongoCollection) shardFilter(shardKeyValue, entityKey interface{}) bson.D {
	return bson.D{
		{Key: this.shardKeyName, Value: shardKeyValue},
		{Key: this.uniqueId, Value: entityKey},
	}
}

// checkShardKeyEnabled 校验分片键附加条件已启用(分片键列与唯一键列均已配置)
// 未启用时返回ErrNoUniqueColumn,防止shardFilter构造出{"":value}之类的错误filter静默匹配不到文档
func (this *MongoCollection) checkShardKeyEnabled() error {
	if len(this.uniqueId) == 0 || len(this.shardKeyName) == 0 {
		return ErrNoUniqueColumn
	}
	return nil
}

// checkShardKeyMatched 写操作执行后的防线校验:分片键filter匹配0条时mongo不报错(静默无操作),
// 但这通常意味着分片键值与文档中该字段的值不一致(配置错误或数据异常),
// 上层的"写库成功后删缓存/重置修改标记"防线会随之失效,造成数据丢失,必须打日志让运维可感知
// NOTE:只告警不报错,不改变既有行为(修复链路依赖写库成功后删缓存,报错会中断后续组件的修复)
func (this *MongoCollection) checkShardKeyMatched(op string, shardKeyValue, entityKey interface{}, matchedCount int64) {
	if matchedCount == 0 {
		glog.Error("ShardKeyFilterMatchedNothing", "op", op, "collection", this.collectionName,
			"shardKey", this.shardKeyName, "shardKeyValue", shardKeyValue, "entityKey", entityKey)
	}
}

// 根据id+分片键查找数据(直达分片)
func (this *MongoCollection) FindEntityByIdWithShardKey(shardKeyValue, entityKey interface{}, data interface{}) (bool, error) {
	if err := this.checkShardKeyEnabled(); err != nil {
		return false, err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.Background(), this.shardFilter(shardKeyValue, entityKey))
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true, nil
}

// 保存Entity数据(直达分片)
func (this *MongoCollection) SaveEntityWithShardKey(shardKeyValue, entityKey interface{}, entityData interface{}) error {
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	res, err := col.UpdateOne(context.Background(), this.shardFilter(shardKeyValue, entityKey), entityData)
	if err != nil {
		return err
	}
	this.checkShardKeyMatched("SaveEntity", shardKeyValue, entityKey, res.MatchedCount)
	return nil
}

// 删除Entity数据(直达分片)
func (this *MongoCollection) DeleteEntityWithShardKey(shardKeyValue, entityKey interface{}) error {
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.DeleteOne(context.Background(), this.shardFilter(shardKeyValue, entityKey))
	return err
}

// 保存1个组件(直达分片)
func (this *MongoCollection) SaveComponentWithShardKey(shardKeyValue, entityKey interface{}, componentName string, componentData interface{}) error {
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	res, updateErr := col.UpdateOne(context.Background(), this.shardFilter(shardKeyValue, entityKey),
		bson.D{{Key: "$set", Value: bson.D{{Key: componentName, Value: componentData}}}})
	if updateErr != nil {
		return updateErr
	}
	this.checkShardKeyMatched("SaveComponent", shardKeyValue, entityKey, res.MatchedCount)
	return nil
}

// 批量保存组件(直达分片)
func (this *MongoCollection) SaveComponentsWithShardKey(shardKeyValue, entityKey interface{}, components map[string]interface{}) error {
	if len(components) == 0 {
		return nil
	}
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	// filter含唯一键,最多匹配1个文档,UpdateOne语义准确
	res, updateErr := col.UpdateOne(context.Background(), this.shardFilter(shardKeyValue, entityKey),
		bson.D{{Key: "$set", Value: components}})
	if updateErr != nil {
		return updateErr
	}
	this.checkShardKeyMatched("SaveComponents", shardKeyValue, entityKey, res.MatchedCount)
	return nil
}

// 保存1个组件的一个字段(直达分片)
func (this *MongoCollection) SaveComponentFieldWithShardKey(shardKeyValue, entityKey interface{}, componentName string, fieldName string, fieldData interface{}) error {
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	// NOTE:如果player.ComponentName == null
	// 直接更新player.ComponentName.fieldName会报错: Cannot create field 'fieldName' in element
	res, updateErr := col.UpdateOne(context.Background(), this.shardFilter(shardKeyValue, entityKey),
		bson.D{{Key: "$set", Value: bson.D{{Key: componentName + "." + fieldName, Value: fieldData}}}})
	if updateErr != nil {
		return updateErr
	}
	this.checkShardKeyMatched("SaveComponentField", shardKeyValue, entityKey, res.MatchedCount)
	return nil
}

// 删除1个组件的某些字段(直达分片)
func (this *MongoCollection) DeleteComponentFieldWithShardKey(shardKeyValue, entityKey interface{}, componentName string, fieldName ...string) error {
	if len(fieldName) == 0 {
		return nil
	}
	if err := this.checkShardKeyEnabled(); err != nil {
		return err
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	fieldNames := bson.D{}
	for _, name := range fieldName {
		fieldNames = append(fieldNames, bson.E{Key: componentName + "." + name})
	}
	res, updateErr := col.UpdateOne(context.Background(), this.shardFilter(shardKeyValue, entityKey),
		bson.D{{Key: "$unset", Value: fieldNames}})
	if updateErr != nil {
		return updateErr
	}
	this.checkShardKeyMatched("DeleteComponentField", shardKeyValue, entityKey, res.MatchedCount)
	return nil
}

// db.PlayerDb的mongo实现
type MongoCollectionPlayer struct {
	MongoCollection
	// 账号id列名(index)
	colAccountId string
	//// 账号名列名(index)
	//colAccountName string
	// 玩家区服id列名
	colRegionId string
}

// 根据账号id查找玩家数据
// 适用于一个账号在一个区服只有一个玩家角色的游戏
func (this *MongoCollectionPlayer) FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.Background(), bson.D{{Key: this.colAccountId, Value: accountId}, {Key: this.colRegionId, Value: regionId}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(playerData)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *MongoCollectionPlayer) FindPlayerIdByAccountId(accountId int64, regionId int32) (int64, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.FindOne().
		SetProjection(bson.D{{Key: this.uniqueId, Value: 1}})
	result := col.FindOne(context.Background(), bson.D{{Key: this.colAccountId, Value: accountId}, {Key: this.colRegionId, Value: regionId}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return 0, nil
	}
	// 解码到bson.M后用类型断言取值,兼容int32/int64/double等多种bson数值类型
	var data bson.M
	if err := result.Decode(&data); err != nil {
		return 0, err
	}
	return toInt64(data[this.uniqueId]), nil
}

func (this *MongoCollectionPlayer) FindPlayerIdsByAccountId(accountId int64, regionId int32) ([]int64, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.Find().
		SetProjection(bson.D{{Key: this.uniqueId, Value: 1}})
	cursor, err := col.Find(context.Background(), bson.D{{Key: this.colAccountId, Value: accountId}, {Key: this.colRegionId, Value: regionId}}, opts)
	if err != nil {
		return nil, err
	}
	var datas []bson.M
	if err = cursor.All(context.Background(), &datas); err != nil {
		return nil, err
	}
	playerIds := make([]int64, len(datas), len(datas))
	for i, data := range datas {
		switch id := data[this.uniqueId].(type) {
		case int64:
			playerIds[i] = id
		case uint64:
			playerIds[i] = int64(id)
		case int:
			playerIds[i] = int64(id)
		case uint:
			playerIds[i] = int64(id)
		case int32:
			playerIds[i] = int64(id)
		case uint32:
			playerIds[i] = int64(id)
		}
	}
	return playerIds, nil
}

func (this *MongoCollectionPlayer) FindAccountIdByPlayerId(playerId int64) (int64, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.FindOne().
		SetProjection(bson.D{{Key: this.colAccountId, Value: 1}})
	result := col.FindOne(context.Background(), bson.D{{Key: this.uniqueId, Value: playerId}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return 0, nil
	}
	// 解码到bson.M后用类型断言取值,兼容int32/int64/double等多种bson数值类型
	var data bson.M
	if err := result.Decode(&data); err != nil {
		return 0, err
	}
	return toInt64(data[this.colAccountId]), nil
}

var _ DbMgr = (*MongoDb)(nil)

// db.DbMgr的mongo实现
type MongoDb struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database

	uri    string
	dbName string

	entityDbs map[string]EntityDb
	kvDbs     map[string]KvDb
}

// NewMongoDb只保存连接参数,不会建立连接
// 标准使用流程(顺序不可颠倒):
// NewMongoDb -> RegisterXxxDb(注册表) -> Connect()(连接+建唯一索引) -> ShardDatabase(可选,分片)
func NewMongoDb(uri, dbName string) *MongoDb {
	return &MongoDb{
		uri:       uri,
		dbName:    dbName,
		entityDbs: make(map[string]EntityDb),
		kvDbs:     make(map[string]KvDb),
	}
}

// 注册普通Entity对应的collection
// shardKeyType指定分片方式,ShardKeyNone表示不分片
func (this *MongoDb) RegisterEntityDb(collectionName string, shardKeyType ShardKeyType, uniqueId string) EntityDb {
	col := &MongoCollection{
		mongoClient:    this.mongoClient,
		mongoDatabase:  this.mongoDatabase,
		shardKeyType:   shardKeyType,
		collectionName: collectionName,
		uniqueId:       uniqueId,
	}
	this.entityDbs[collectionName] = col
	glog.Info("RegisterEntityDb", "collection", collectionName, "uniqueId", uniqueId, "shardKeyType", shardKeyType)
	return col
}

// 注册玩家对应的collection
// shardKeyType指定分片方式,ShardKeyNone表示不分片
func (this *MongoDb) RegisterPlayerDb(collectionName string, shardKeyType ShardKeyType, playerId, accountId, region string) PlayerDb {
	col := &MongoCollectionPlayer{
		MongoCollection: MongoCollection{
			mongoClient:    this.mongoClient,
			mongoDatabase:  this.mongoDatabase,
			shardKeyType:   shardKeyType,
			collectionName: collectionName,
			uniqueId:       playerId,
		},
		colAccountId: accountId,
		colRegionId:  region,
	}
	this.entityDbs[collectionName] = col
	glog.Info("RegisterPlayerDb", "collection", collectionName, "playerId", playerId, "shardKeyType", shardKeyType)
	return col
}

// NOTE: 必须在Connect()之前调用,原因见RegisterEntityDb
func (this *MongoDb) RegisterKvDb(collectionName string, shardKeyType ShardKeyType, keyName, valueName string) KvDb {
	col := &MongoKvDb{
		mongoDatabase:  this.mongoDatabase,
		shardKeyType:   shardKeyType,
		collectionName: collectionName,
		keyName:        keyName,
		valueName:      valueName,
	}
	this.kvDbs[collectionName] = col
	glog.Info("RegisterKvDb", "collection", collectionName, "key", keyName, "value", valueName, "shardKeyType", shardKeyType)
	return col
}

func (this *MongoDb) GetEntityDb(name string) EntityDb {
	return this.entityDbs[name]
}

func (this *MongoDb) GetKvDb(name string) KvDb {
	return this.kvDbs[name]
}

func (this *MongoDb) Connect() bool {
	client, err := mongo.Connect(options.Client().ApplyURI(this.uri))
	if err != nil {
		glog.Error("ConnectError", "err", err)
		return false
	}
	// Ping the primary
	if err = client.Ping(context.Background(), readpref.Primary()); err != nil {
		glog.Error("PingError", "err", err)
		return false
	}
	this.mongoClient = client
	this.mongoDatabase = this.mongoClient.Database(this.dbName)
	for _, entityDb := range this.entityDbs {
		switch mongoCollection := entityDb.(type) {
		case *MongoCollection:
			mongoCollection.mongoClient = this.mongoClient
			mongoCollection.mongoDatabase = this.mongoDatabase
			if mongoCollection.uniqueId != "" && mongoCollection.uniqueId != "_id" {
				mongoCollection.CreateIndex(mongoCollection.uniqueId, true)
			}

		case *MongoCollectionPlayer:
			mongoCollection.mongoClient = this.mongoClient
			mongoCollection.mongoDatabase = this.mongoDatabase
			if mongoCollection.uniqueId != "" && mongoCollection.uniqueId != "_id" {
				mongoCollection.CreateIndex(mongoCollection.uniqueId, true)
			}
		}
	}

	for _, kvDb := range this.kvDbs {
		switch mongoCollection := kvDb.(type) {
		case *MongoKvDb:
			mongoCollection.mongoDatabase = this.mongoDatabase
			if mongoCollection.keyName != "" && mongoCollection.keyName != "_id" {
				indexModel := mongo.IndexModel{
					Keys:    bson.D{{Key: mongoCollection.keyName, Value: 1}},
					Options: options.Index().SetUnique(true),
				}
				col := this.mongoDatabase.Collection(mongoCollection.collectionName)
				indexName, indexErr := col.Indexes().CreateOne(context.Background(), indexModel)
				if indexErr != nil {
					glog.Error("create index", "collection", mongoCollection.collectionName, "index", indexName, "err", indexErr)
				} else {
					glog.Info("index", "collection", mongoCollection.collectionName, "indexName", indexName)
				}
			}
		}
	}

	glog.Info("mongo Connected")
	return true
}

func (this *MongoDb) Disconnect() {
	if this.mongoClient == nil {
		return
	}
	if err := this.mongoClient.Disconnect(context.Background()); err != nil {
		glog.Error("DisconnectError", "err", err)
	}
	glog.Info("mongo Disconnected")
}

func (this *MongoDb) GetMongoDatabase() *mongo.Database {
	return this.mongoDatabase
}

func (this *MongoDb) GetMongoClient() *mongo.Client {
	return this.mongoClient
}

// 设置database分片
// NOTE:
// 1. 需在Connect()之后调用
// 2. 只对当前已注册的collection生效,注册在后的表不会被分片
// 3. ShardKeyNone的collection会自动跳过
func (this *MongoDb) ShardDatabase(dbName string) error {
	adminDb := this.mongoClient.Database("admin")
	err := adminDb.RunCommand(context.Background(), bson.D{
		{Key: "enableSharding", Value: dbName},
	}).Err()
	if err != nil {
		// 单机部署的mongodb,会报错no such command: 'enableSharding'
		return err
	}
	for _, entityDb := range this.entityDbs {
		if shard, ok := entityDb.(Sharding); ok {
			shard.Shard()
		}
	}
	for _, kvDb := range this.kvDbs {
		if shard, ok := kvDb.(Sharding); ok {
			shard.Shard()
		}
	}
	return err
}

// 设置collection分片
func (this *MongoDb) ShardCollection(collectionFullName, keyName string, shardKeyType ShardKeyType) error {
	if shardKeyType == ShardKeyNone {
		return nil
	}
	adminDb := this.mongoClient.Database("admin")
	key := bson.E{Key: keyName, Value: 1}
	if shardKeyType == ShardKeyHashed {
		key.Value = "hashed"
	}
	err := adminDb.RunCommand(context.Background(), bson.D{
		{Key: "shardCollection", Value: collectionFullName},
		{Key: "key", Value: bson.D{key}},
	}).Err()
	if err != nil {
		glog.Error("ShardCollectionError", "collection", collectionFullName, "err", err)
	}
	return err
}

// 检查是否是key重复错误
func IsDuplicateKeyError(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

// 把bson解码后的整数值统一转成int64
// bson中数值可能是int32/int64/double等,直接调用Int64()会在类型不匹配时报错;
// 解码到bson.M后用类型断言统一处理,更加健壮
func toInt64(v any) int64 {
	switch id := v.(type) {
	case int64:
		return id
	case uint64:
		return int64(id)
	case int:
		return int64(id)
	case uint:
		return int64(id)
	case int32:
		return int64(id)
	case uint32:
		return int64(id)
	case float64:
		return int64(id)
	case float32:
		return int64(id)
	}
	return 0
}
