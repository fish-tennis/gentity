package gentity

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ PlayerDb = (*MongoCollectionPlayer)(nil)
var _ EntityDb = (*MongoCollection)(nil)

// db.EntityDb的mongo实现
type MongoCollection struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database

	// 表名
	collectionName string
	// 唯一id
	uniqueId string
}

func (this *MongoCollection) GetCollection() *mongo.Collection {
	return this.mongoDatabase.Collection(this.collectionName)
}

func (this *MongoCollection) CreateIndex(key string, unique bool) {
	col := this.mongoDatabase.Collection(this.collectionName)
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{key, 1},
		},
		Options: options.Index().SetUnique(unique),
	}
	indexName, indexErr := col.Indexes().CreateOne(context.Background(), indexModel)
	if indexErr != nil {
		GetLogger().Error("%v create index %v err:%v", this.collectionName, indexName, indexErr)
	} else {
		GetLogger().Info("%v index:%v", this.collectionName, indexName)
	}
}

// 设置分片key
func (this *MongoCollection) ShardCollection(hashedShardKey bool) error {
	collectionFullName := fmt.Sprintf("%v.%v", this.mongoDatabase.Name(), this.collectionName)
	key := bson.E{Key: this.uniqueId, Value: 1}
	if hashedShardKey {
		key.Value = "hashed"
	}
	err := this.mongoClient.Database("admin").RunCommand(context.Background(), bson.D{
		{"shardCollection", collectionFullName},
		{"key", bson.D{key}},
	}).Err()
	if err != nil {
		GetLogger().Error("ShardCollection %v err:%v", collectionFullName, err)
	} else {
		GetLogger().Info("ShardCollection %v hashed:%v", collectionFullName, hashedShardKey)
	}
	return err
}

// 根据id查找数据
func (this *MongoCollection) FindEntityById(entityId int64, data interface{}) (bool, error) {
	if len(this.uniqueId) == 0 {
		return false, errors.New("no uniqueId column")
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.Background(), bson.D{{this.uniqueId, entityId}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	err := result.Decode(data)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (this *MongoCollection) InsertEntity(entityId int64, entityData interface{}) (err error, isDuplicateKey bool) {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err = col.InsertOne(context.Background(), entityData)
	if err != nil {
		isDuplicateKey = IsDuplicateKeyError(err)
	}
	return
}

func (this *MongoCollection) SaveEntity(entityId int64, entityData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.UpdateOne(context.Background(), bson.D{{this.uniqueId, entityId}}, entityData)
	return err
}

func (this *MongoCollection) SaveComponent(entityId int64, componentName string, componentData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{this.uniqueId, entityId}},
		bson.D{{"$set", bson.D{{componentName, componentData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoCollection) SaveComponents(entityId int64, components map[string]interface{}) error {
	if len(components) == 0 {
		return nil
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	_, updateErr := col.UpdateMany(context.Background(), bson.D{{this.uniqueId, entityId}},
		bson.D{{"$set", components}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func (this *MongoCollection) SaveComponentField(entityId int64, componentName string, fieldName string, fieldData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	// NOTE:如果player.componentName == null
	// 直接更新player.componentName.fieldName会报错: Cannot create field 'fieldName' in element
	_, updateErr := col.UpdateOne(context.Background(), bson.D{{this.uniqueId, entityId}},
		bson.D{{"$set", bson.D{{componentName + "." + fieldName, fieldData}}}})
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// 删除1个组件的某些字段
func (this *MongoCollection) DeleteComponentField(entityId int64, componentName string, fieldName ...string) error {
	if len(fieldName) == 0 {
		return nil
	}
	col := this.mongoDatabase.Collection(this.collectionName)
	fieldNames := bson.D{}
	for _, name := range fieldName {
		fieldNames = append(fieldNames, bson.E{Key: componentName + "." + name})
	}
	result, updateErr := col.UpdateOne(context.Background(), bson.D{{this.uniqueId, entityId}},
		bson.D{{"$unset", fieldNames}})
	if updateErr != nil {
		return updateErr
	}
	GetLogger().Debug("%v", result)
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
	result := col.FindOne(context.Background(), bson.D{{this.colAccountId, accountId}, {this.colRegionId, regionId}})
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
		SetProjection(bson.D{{this.uniqueId, 1}})
	result := col.FindOne(context.Background(), bson.D{{this.colAccountId, accountId}, {this.colRegionId, regionId}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return 0, nil
	}
	res, err := result.DecodeBytes()
	if err != nil {
		return 0, err
	}
	idValue, err := res.LookupErr(this.uniqueId)
	if err != nil {
		return 0, err
	}
	return idValue.Int64(), nil
}

func (this *MongoCollectionPlayer) FindPlayerIdsByAccountId(accountId int64, regionId int32) ([]int64, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.Find().
		SetProjection(bson.D{{this.uniqueId, 1}})
	cursor, err := col.Find(context.Background(), bson.D{{this.colAccountId, accountId}, {this.colRegionId, regionId}}, opts)
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
		SetProjection(bson.D{{this.colAccountId, 1}})
	result := col.FindOne(context.Background(), bson.D{{this.uniqueId, playerId}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return 0, nil
	}
	res, err := result.DecodeBytes()
	if err != nil {
		return 0, err
	}
	idValue, err := res.LookupErr(this.colAccountId)
	if err != nil {
		return 0, err
	}
	return idValue.Int64(), nil
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

func NewMongoDb(uri, dbName string) *MongoDb {
	return &MongoDb{
		uri:       uri,
		dbName:    dbName,
		entityDbs: make(map[string]EntityDb),
		kvDbs:     make(map[string]KvDb),
	}
}

// 注册普通Entity对应的collection
func (this *MongoDb) RegisterEntityDb(collectionName string, uniqueId string) EntityDb {
	col := &MongoCollection{
		mongoClient:    this.mongoClient,
		mongoDatabase:  this.mongoDatabase,
		collectionName: collectionName,
		uniqueId:       uniqueId,
	}
	this.entityDbs[collectionName] = col
	GetLogger().Info("RegisterEntityDb %v %v", collectionName, uniqueId)
	return col
}

// 注册玩家对应的collection
func (this *MongoDb) RegisterPlayerDb(collectionName string, playerId, accountId, region string) PlayerDb {
	col := &MongoCollectionPlayer{
		MongoCollection: MongoCollection{
			mongoClient:    this.mongoClient,
			mongoDatabase:  this.mongoDatabase,
			collectionName: collectionName,
			uniqueId:       playerId,
		},
		colAccountId: accountId,
		colRegionId:  region,
	}
	this.entityDbs[collectionName] = col
	GetLogger().Info("RegisterPlayerDb %v %v", collectionName, playerId)
	return col
}

func (this *MongoDb) RegisterKvDb(collectionName, keyName, valueName string) KvDb {
	col := &MongoKvDb{
		mongoDatabase:  this.mongoDatabase,
		collectionName: collectionName,
		keyName:        keyName,
		valueName:      valueName,
	}
	this.kvDbs[collectionName] = col
	GetLogger().Info("RegisterKvDb %v %v %v", collectionName, keyName, valueName)
	return col
}

func (this *MongoDb) GetEntityDb(name string) EntityDb {
	return this.entityDbs[name]
}

func (this *MongoDb) GetKvDb(name string) KvDb {
	return this.kvDbs[name]
}

func (this *MongoDb) Connect() bool {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(this.uri))
	if err != nil {
		return false
	}
	// Ping the primary
	if err := client.Ping(context.Background(), readpref.Primary()); err != nil {
		GetLogger().Error(err.Error())
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
					Keys:    bson.D{{mongoCollection.keyName, 1}},
					Options: options.Index().SetUnique(true),
				}
				col := this.mongoDatabase.Collection(mongoCollection.collectionName)
				indexName, indexErr := col.Indexes().CreateOne(context.Background(), indexModel)
				if indexErr != nil {
					GetLogger().Error("%v create index %v err:%v", mongoCollection.collectionName, indexName, indexErr)
				} else {
					GetLogger().Info("%v index:%v", mongoCollection.collectionName, indexName)
				}
			}
		}
	}

	GetLogger().Info("mongo Connected")
	return true
}

func (this *MongoDb) Disconnect() {
	if this.mongoClient == nil {
		return
	}
	if err := this.mongoClient.Disconnect(context.Background()); err != nil {
		GetLogger().Error(err.Error())
	}
	GetLogger().Info("mongo Disconnected")
}

func (this *MongoDb) GetMongoDatabase() *mongo.Database {
	return this.mongoDatabase
}

func (this *MongoDb) GetMongoClient() *mongo.Client {
	return this.mongoClient
}

// 设置database分片
func (this *MongoDb) ShardDatabase(dbName string) error {
	adminDb := this.mongoClient.Database("admin")
	return adminDb.RunCommand(context.Background(), bson.D{
		{"enableSharding", dbName},
	}).Err()
}

// 设置database分片
func (this *MongoDb) ShardCollection(collectionFullName, keyName string, hashedShardKey bool) error {
	adminDb := this.mongoClient.Database("admin")
	key := bson.E{Key: keyName, Value: 1}
	if hashedShardKey {
		key.Value = "hashed"
	}
	err := adminDb.RunCommand(context.Background(), bson.D{
		{"shardCollection", collectionFullName},
		{"key", bson.D{key}},
	}).Err()
	if err != nil {
		GetLogger().Error("ShardCollection %v err:%v", collectionFullName, err)
	}
	return err
}

// 检查是否是key重复错误
func IsDuplicateKeyError(err error) bool {
	switch e := err.(type) {
	case mongo.WriteException:
		for _, writeErr := range e.WriteErrors {
			if writeErr.Code == 11000 {
				return true
			}
		}
	}
	return false
}
