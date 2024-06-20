package gentity

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
	"time"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ KvCache = (*RedisCache)(nil)

// KvCache的redis实现
type RedisCache struct {
	redisClient redis.Cmdable
}

func NewRedisCache(redisClient redis.Cmdable) *RedisCache {
	return &RedisCache{
		redisClient: redisClient,
	}
}

func ignoreNilError(redisError error) error {
	if IsRedisError(redisError) {
		return redisError
	}
	return nil
}

func (this *RedisCache) Get(key string) (string, error) {
	data, err := this.redisClient.Get(context.Background(), key).Result()
	return data, ignoreNilError(err)
}

func (this *RedisCache) Set(key string, value interface{}, expiration time.Duration) error {
	// 如果是proto,自动转换成[]byte
	if protoMessage, ok := value.(proto.Message); ok {
		bytes, protoErr := proto.Marshal(protoMessage)
		if protoErr != nil {
			return protoErr
		}
		_, err := this.redisClient.Set(context.Background(), key, bytes, expiration).Result()
		return ignoreNilError(err)
	}
	_, err := this.redisClient.Set(context.Background(), key, value, expiration).Result()
	return ignoreNilError(err)
}

func (this *RedisCache) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	// 如果是proto,自动转换成[]byte
	if protoMessage, ok := value.(proto.Message); ok {
		bytes, protoErr := proto.Marshal(protoMessage)
		if protoErr != nil {
			return false, protoErr
		}
		isSetOk, err := this.redisClient.SetNX(context.Background(), key, bytes, expiration).Result()
		return isSetOk, ignoreNilError(err)
	}
	isSetOk, err := this.redisClient.SetNX(context.Background(), key, value, expiration).Result()
	return isSetOk, ignoreNilError(err)
}

func (this *RedisCache) Del(key ...string) (int64, error) {
	delCount, err := this.redisClient.Del(context.Background(), key...).Result()
	return delCount, ignoreNilError(err)
}

func (this *RedisCache) Type(key string) (string, error) {
	data, err := this.redisClient.Type(context.Background(), key).Result()
	return data, ignoreNilError(err)
}

// redis hash -> map
func (this *RedisCache) GetMap(key string, m interface{}) error {
	if m == nil {
		return errors.New(fmt.Sprintf("map must valid key:%v", key))
	}
	strMap, err := this.redisClient.HGetAll(context.Background(), key).Result()
	if IsRedisError(err) {
		return err
	}
	val := reflect.ValueOf(m)
	if val.Kind() != reflect.Map {
		return errors.New(fmt.Sprintf("unsupport type kind:%v key:%v", val.Kind(), key))
	}
	typ := reflect.TypeOf(m)
	keyType := typ.Key()
	valType := typ.Elem()
	for k, v := range strMap {
		realKey := ConvertStringToRealType(keyType, k)
		// 如果是map是map[string]any,value解析需要特殊处理
		realValue := ConvertStringToRealType(valType, v)
		val.SetMapIndex(reflect.ValueOf(realKey), reflect.ValueOf(realValue))
	}
	return nil
}

// map -> redis hash
func (this *RedisCache) SetMap(k string, m interface{}) error {
	cacheData := make(map[string]interface{})
	val := reflect.ValueOf(m)
	it := val.MapRange()
	for it.Next() {
		key, err := convertValueToString(it.Key())
		if err != nil {
			return err
		}
		value, err := convertValueToStringOrInterface(it.Value())
		if err != nil {
			return err
		}
		cacheData[key] = value
	}
	if len(cacheData) == 0 {
		return nil
	}
	_, err := this.redisClient.HSet(context.Background(), k, cacheData).Result()
	return ignoreNilError(err)
}

func (this *RedisCache) HGetAll(key string) (map[string]string, error) {
	m, err := this.redisClient.HGetAll(context.Background(), key).Result()
	return m, ignoreNilError(err)
}

func (this *RedisCache) HSet(key string, values ...interface{}) (int64, error) {
	count, redisError := this.redisClient.HSet(context.Background(), key, values...).Result()
	return count, ignoreNilError(redisError)
}

func (this *RedisCache) HSetNX(key, field string, value interface{}) (bool, error) {
	return this.redisClient.HSetNX(context.Background(), key, field, value).Result()
}

func (this *RedisCache) HDel(key string, fields ...string) (int64, error) {
	delCount, err := this.redisClient.HDel(context.Background(), key, fields...).Result()
	return delCount, ignoreNilError(err)
}

func (this *RedisCache) GetProto(key string, value proto.Message) error {
	str, err := this.redisClient.Get(context.Background(), key).Result()
	// 不存在的key或者空数据,直接跳过,防止错误的覆盖
	if err == redis.Nil || len(str) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	err = proto.Unmarshal([]byte(str), value)
	return err
}

// 检查redis返回的error是否是异常
func IsRedisError(redisError error) bool {
	// redis的key不存在,会返回redis.Nil,但是不是我们常规认为的error(异常),所以要忽略redis.Nil
	if redisError != nil && redisError != redis.Nil {
		return true
	}
	return false
}
