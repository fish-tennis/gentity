package gentity

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// KvDb的mongo实现
type MongoKvDb struct {
	mongoDatabase *mongo.Database

	// 表名
	collectionName string
	// key column name
	keyName string
	// value column name
	valueName string
}

func (this *MongoKvDb) GetCollection() *mongo.Collection {
	return this.mongoDatabase.Collection(this.collectionName)
}

func (this *MongoKvDb) Find(key interface{}) (interface{}, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	result := col.FindOne(context.Background(), bson.D{{this.keyName, key}})
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return nil, nil
	}
	var doc bson.M
	err := result.Decode(&doc)
	if err != nil {
		return nil, err
	}
	return doc[this.valueName], nil
}

func (this *MongoKvDb) FindAndDecode(key interface{}, decodeData interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	opts := options.FindOne().
		SetProjection(bson.D{{this.valueName, 1}})
	result := col.FindOne(context.Background(), bson.D{{this.keyName, key}}, opts)
	if result == nil || result.Err() == mongo.ErrNoDocuments {
		return nil
	}
	raw,err := result.DecodeBytes()
	if err != nil {
		return err
	}
	err = raw.Lookup(this.valueName).Unmarshal(decodeData)
	return err
}

func (this *MongoKvDb) Insert(key interface{}, value interface{}) (err error, isDuplicateKey bool) {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err = col.InsertOne(context.Background(),
		bson.D{{this.keyName, key}, {this.valueName, value}})
	if err != nil {
		isDuplicateKey = IsDuplicateKeyError(err)
	}
	return
}

func (this *MongoKvDb) Update(key interface{}, value interface{}, upsert bool) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	opt := options.Update().SetUpsert(upsert)
	_, err := col.UpdateOne(context.Background(),
		bson.D{{this.keyName, key}},
		bson.D{{"$set", bson.D{{this.valueName, value}}}},
		opt)
	return err
}

func (this *MongoKvDb) Inc(key interface{}, value interface{}, upsert bool) (interface{}, error) {
	col := this.mongoDatabase.Collection(this.collectionName)
	opt := options.FindOneAndUpdate().SetUpsert(upsert).SetReturnDocument(options.After)
	updateResult := col.FindOneAndUpdate(context.Background(),
		bson.D{{this.keyName, key}},
		bson.D{{"$inc", bson.D{{this.valueName, value}}}},
		opt)
	if updateResult.Err() != nil {
		return nil, updateResult.Err()
	}
	var updatedDocument bson.M
	updateResult.Decode(&updatedDocument)
	return updatedDocument[this.valueName], nil
}

func (this *MongoKvDb) Delete(key interface{}) error {
	col := this.mongoDatabase.Collection(this.collectionName)
	_, err := col.DeleteOne(context.Background(), bson.D{{this.keyName, key}})
	return err
}
