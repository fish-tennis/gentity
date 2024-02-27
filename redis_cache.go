package gentity

import (
	"context"
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
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
		realKey := convertStringToRealType(keyType, k)
		realValue := convertStringToRealType(valType, v)
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

func convertValueToString(val reflect.Value) (string, error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.Itoa(int(val.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(val.Float(), 'f', 2, 64), nil
	case reflect.String:
		return val.String(), nil
	case reflect.Interface:
		if !val.CanInterface() {
			GetLogger().Error("unsupport type:%v", val.Kind())
			return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
		}
		return util.ToString(val.Interface())
	default:
		GetLogger().Error("unsupport type:%v", val.Kind())
		return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
}

func convertValueToStringOrInterface(val reflect.Value) (interface{}, error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return convertValueToString(val)
	case reflect.Interface, reflect.Ptr:
		if !val.IsNil() {
			if !val.CanInterface() {
				GetLogger().Error("unsupport type:%v", val.Kind())
				return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
			}
			i := val.Interface()
			// protobuf格式
			if protoMessage, ok := i.(proto.Message); ok {
				bytes, protoErr := proto.Marshal(protoMessage)
				if protoErr != nil {
					GetLogger().Error("convert proto err:%v", protoErr.Error())
					return nil, protoErr
				}
				return bytes, nil
			}
			// Saveable格式
			if valueSaveable, ok := i.(Saveable); ok {
				valueSaveData, valueSaveErr := GetSaveData(valueSaveable, "")
				if valueSaveErr != nil {
					GetLogger().Error("convert Saveabl err:%v", valueSaveErr.Error())
					return nil, valueSaveErr
				}
				return valueSaveData, nil
			}
			return i, nil
		}
	default:
		GetLogger().Error("unsupport type:%v", val.Kind())
		return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
	GetLogger().Error("unsupport type:%v", val.Kind())
	return nil, errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
}

func convertStringToRealType(typ reflect.Type, v string) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return util.Atoi(v)
	case reflect.Int8:
		return int8(util.Atoi(v))
	case reflect.Int16:
		return int16(util.Atoi(v))
	case reflect.Int32:
		return int32(util.Atoi(v))
	case reflect.Int64:
		return util.Atoi64(v)
	case reflect.Uint:
		return uint(util.Atou(v))
	case reflect.Uint8:
		return uint8(util.Atou(v))
	case reflect.Uint16:
		return uint16(util.Atou(v))
	case reflect.Uint32:
		return uint32(util.Atou(v))
	case reflect.Uint64:
		return util.Atou(v)
	case reflect.Float32:
		f, _ := strconv.ParseFloat(v, 64)
		return float32(f)
	case reflect.Float64:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case reflect.String:
		return v
	case reflect.Interface, reflect.Ptr:
		newProto := reflect.New(typ.Elem())
		if protoMessage, ok := newProto.Interface().(proto.Message); ok {
			protoErr := proto.Unmarshal([]byte(v), protoMessage)
			if protoErr != nil {
				GetLogger().Error("proto err:%v", protoErr.Error())
				return protoErr
			}
			return protoMessage
		}
	}
	GetLogger().Error("unsupport type:%v", typ.Kind())
	return nil
}

// 检查redis返回的error是否是异常
func IsRedisError(redisError error) bool {
	// redis的key不存在,会返回redis.Nil,但是不是我们常规认为的error(异常),所以要忽略redis.Nil
	if redisError != nil && redisError != redis.Nil {
		return true
	}
	return false
}
