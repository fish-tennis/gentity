package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/logger"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
	"time"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ gentity.KvCache = (*RedisCache)(nil)

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
	data,err := this.redisClient.Get(context.Background(), key).Result()
	return data,ignoreNilError(err)
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

func (this *RedisCache) Del(key ...string) error {
	_, err := this.redisClient.Del(context.Background(), key...).Result()
	return ignoreNilError(err)
}

func (this *RedisCache) Type(key string) (string, error) {
	data,err := this.redisClient.Type(context.Background(), key).Result()
	return data,ignoreNilError(err)
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
		key,err := convertValueToString(it.Key())
		if err != nil {
			return err
		}
		value,err := convertValueToStringOrInterface(it.Value())
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

func (this *RedisCache) SetMapField(key, fieldName string, value interface{}) (isNewField bool, err error) {
	ret, redisError := this.redisClient.HSet(context.Background(), key, fieldName, value).Result()
	return ret == 1, ignoreNilError(redisError)
}

func (this *RedisCache) DelMapField(key string, fields ...string) error {
	_, err := this.redisClient.HDel(context.Background(), key, fields...).Result()
	return ignoreNilError(err)
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

func (this *RedisCache) SetProto(key string, value proto.Message, expiration time.Duration) error {
	bytes, protoErr := proto.Marshal(value)
	if protoErr != nil {
		return protoErr
	}
	_, err := this.redisClient.Set(context.Background(), key, bytes, expiration).Result()
	return ignoreNilError(err)
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
			logger.Error("unsupport type:%v", val.Kind())
			return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
		}
		return gentity.ToString(val.Interface())
	default:
		logger.Error("unsupport type:%v", val.Kind())
		return "", errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
}

func convertValueToStringOrInterface(val reflect.Value) (interface{},error) {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return convertValueToString(val)
	case reflect.Interface, reflect.Ptr:
		if !val.IsNil() {
			if !val.CanInterface() {
				logger.Error("unsupport type:%v", val.Kind())
				return nil,errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
			}
			i := val.Interface()
			if protoMessage, ok := i.(proto.Message); ok {
				bytes, protoErr := proto.Marshal(protoMessage)
				if protoErr != nil {
					logger.Error("proto err:%v", protoErr.Error())
					return nil, protoErr
				}
				return bytes,nil
			}
			return i,nil
		}
	default:
		logger.Error("unsupport type:%v", val.Kind())
		return nil,errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
	}
	logger.Error("unsupport type:%v", val.Kind())
	return nil,errors.New(fmt.Sprintf("unsupport type:%v", val.Kind()))
}

func convertStringToRealType(typ reflect.Type, v string) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return gentity.Atoi(v)
	case reflect.Int8:
		return int8(gentity.Atoi(v))
	case reflect.Int16:
		return int16(gentity.Atoi(v))
	case reflect.Int32:
		return int32(gentity.Atoi(v))
	case reflect.Int64:
		return gentity.Atoi64(v)
	case reflect.Uint:
		return uint(gentity.Atou(v))
	case reflect.Uint8:
		return uint8(gentity.Atou(v))
	case reflect.Uint16:
		return uint16(gentity.Atou(v))
	case reflect.Uint32:
		return uint32(gentity.Atou(v))
	case reflect.Uint64:
		return gentity.Atou(v)
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
				logger.Error("proto err:%v", protoErr.Error())
				return protoErr
			}
			return protoMessage
		}
	}
	logger.Error("unsupport type:%v", typ.Kind())
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