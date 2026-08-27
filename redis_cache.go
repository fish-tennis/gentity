package gentity

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ KvCache = (*RedisCache)(nil)
var _ ScriptKvCache = (*RedisCache)(nil)

// KvCache的redis实现
type RedisCache struct {
	redisClient redis.Cmdable
	// Redis Function执行器:见redis_function.go
	// 优先FCALL,不可用(Redis<7.0或库未安装)自动回退EVAL脚本
	funcRunner *FunctionRunner
}

func NewRedisCache(redisClient redis.Cmdable) *RedisCache {
	return &RedisCache{
		redisClient: redisClient,
		funcRunner:  NewFunctionRunner(redisClient, functionEvalScripts),
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
	return fillMapFromStringMap(m, strMap)
}

// fillMapFromStringMap 把map[string]string填充到类型明确的map(反射转换)
// m必须是一个类型明确有效的map,key/value类型由ConvertStringToRealType支持
// (int/uint/float/bool/string/[]byte/proto.Message等)
func fillMapFromStringMap(m interface{}, strMap map[string]string) error {
	val := reflect.ValueOf(m)
	if val.Kind() != reflect.Map {
		return errors.New(fmt.Sprintf("unsupport type kind:%v", val.Kind()))
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
	cacheData, err := convertMapToStringMap(m)
	if err != nil {
		return err
	}
	if len(cacheData) == 0 {
		return nil
	}
	_, err = this.redisClient.HSet(context.Background(), k, cacheData).Result()
	return ignoreNilError(err)
}

// convertMapToStringMap 把map转换成redis hash格式的map[string]interface{}
// map的key/value类型支持:string,int,uint,float,bool,complex,proto.Message
func convertMapToStringMap(m interface{}) (map[string]interface{}, error) {
	cacheData := make(map[string]interface{})
	val := reflect.ValueOf(m)
	if val.Kind() != reflect.Map {
		return nil, errors.New(fmt.Sprintf("unsupport type kind:%v", val.Kind()))
	}
	it := val.MapRange()
	for it.Next() {
		key, err := convertValueToString(it.Key())
		if err != nil {
			return nil, err
		}
		value, err := convertValueToStringOrInterface(it.Value())
		if err != nil {
			return nil, err
		}
		cacheData[key] = value
	}
	return cacheData, nil
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

// ==================== Lua脚本原子操作 ====================
// 集群不支持事务(MULTI/EXEC跨节点),但同一节点上的Lua脚本是原子执行的,可达到事务的效果
// 以下脚本均只操作KEYS[1]这一个key,天然满足Redis Cluster"所有key在同一个slot"的限制
// 使用redis.NewScript:优先EVALSHA,脚本未缓存时自动降级EVAL并重试

var (
	// 原子的 DEL + 批量HSET,等价于 Del(key)+SetMap(key,m)
	// KEYS[1]: 目标hash key
	// ARGV: 偶数个,[field1,value1,field2,value2,...]
	// 返回: 恒为1
	luaReplaceMap = redis.NewScript(`
redis.call('DEL', KEYS[1])
for i = 1, #ARGV, 2 do
	redis.call('HSET', KEYS[1], ARGV[i], ARGV[i+1])
end
return 1
`)

	// 原子增量更新map:批量HSET与批量HDEL在同一脚本中执行,等价于 SetMap+HDel
	// KEYS[1]: 目标hash key
	// ARGV[1]: 写入的field数量N(用于区分后面的field/value对与待删除field)
	// ARGV[2]..ARGV[2N+1]: N个field/value对
	// ARGV[2N+2]..ARGV末尾: 待删除的field列表(可为空)
	// 返回: 恒为1
	luaUpdateMap = redis.NewScript(`
local n = tonumber(ARGV[1])
for i = 1, n do
	redis.call('HSET', KEYS[1], ARGV[2*i], ARGV[2*i+1])
end
for i = 2*n+2, #ARGV do
	redis.call('HDEL', KEYS[1], ARGV[i])
end
return 1
`)

	// 条件写入hash field:field不存在或当前值等于指定值时才写入
	// 用于可重入的分布式锁:服务器重启后可重新获取自己上次崩溃残留的锁
	// KEYS[1]: 目标hash key
	// ARGV[1]: field名
	// ARGV[2]: 要写入的值(锁持有者标识)
	// 返回: 1=写入成功(含重入) 0=field已被其他值持有
	luaHSetIfAbsentOrEqual = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], ARGV[1])
if cur == false or cur == ARGV[2] then
	redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
	return 1
end
return 0
`)

	// 条件删除hash field:仅当field的值等于期望值时才删除
	// 防止基于旧状态的无条件删除误删其他持有者(如新持有者)已更新的数据
	// KEYS[1]: 目标hash key
	// ARGV[1]: field名
	// ARGV[2]: 期望值(仅当前值等于它才删除)
	// 返回: 1=已删除 0=值不匹配未删除
	luaHDelIfValueEqual = redis.NewScript(`
if redis.call('HGET', KEYS[1], ARGV[1]) == ARGV[2] then
	return redis.call('HDEL', KEYS[1], ARGV[1])
end
return 0
`)

	// 删除所有值等于指定值的field:等价于 HGetAll+条件HDel 的原子执行
	// 消除快照与删除之间其他持有者重新写入后被误删的竞态
	// KEYS[1]: 目标hash key
	// ARGV[1]: 期望值(值等于它的field才会被删除)
	// 返回: 删除的field数量
	luaHDelFieldsByValue = redis.NewScript(`
local all = redis.call('HGETALL', KEYS[1])
local n = 0
for i = 1, #all, 2 do
	if all[i+1] == ARGV[1] then
		redis.call('HDEL', KEYS[1], all[i])
		n = n + 1
	end
end
return n
`)
)

func (this *RedisCache) AtomicReplaceMap(key string, m interface{}) error {
	cacheData, err := convertMapToStringMap(m)
	if err != nil {
		return err
	}
	if len(cacheData) == 0 {
		// 空map等价于Del+空写入,即删除旧数据
		_, err := this.Del(key)
		return err
	}
	args := make([]interface{}, 0, len(cacheData)*2)
	for k, v := range cacheData {
		args = append(args, k, v)
	}
	_, err = this.runFunction(funcReplaceMap, []string{key}, args...)
	return ignoreNilError(err)
}

func (this *RedisCache) AtomicUpdateMap(key string, setMap interface{}, delFields []string) error {
	var cacheData map[string]interface{}
	if setMap != nil {
		var err error
		cacheData, err = convertMapToStringMap(setMap)
		if err != nil {
			return err
		}
	}
	if len(cacheData) == 0 && len(delFields) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(cacheData)*2+len(delFields)+1)
	args = append(args, len(cacheData))
	for k, v := range cacheData {
		args = append(args, k, v)
	}
	for _, field := range delFields {
		args = append(args, field)
	}
	_, err := this.runFunction(funcUpdateMap, []string{key}, args...)
	return ignoreNilError(err)
}

// scriptIntResult 把Lua脚本的返回值转换成int64
// Lua的整数返回值经redis协议返回后是int64
func scriptIntResult(res interface{}) int64 {
	if n, ok := res.(int64); ok {
		return n
	}
	return 0
}

func (this *RedisCache) HSetIfAbsentOrEqual(key, field string, value interface{}) (bool, error) {
	res, err := this.runFunction(funcHSetIfAbsentOrEqual, []string{key}, field, value)
	if err := ignoreNilError(err); err != nil {
		return false, err
	}
	return scriptIntResult(res) == 1, nil
}

func (this *RedisCache) HDelIfValueEqual(key, field string, expectValue interface{}) (bool, error) {
	res, err := this.runFunction(funcHDelIfValueEqual, []string{key}, field, expectValue)
	if err := ignoreNilError(err); err != nil {
		return false, err
	}
	return scriptIntResult(res) == 1, nil
}

func (this *RedisCache) HDelFieldsByValue(key string, value interface{}) (int64, error) {
	res, err := this.runFunction(funcHDelFieldsByValue, []string{key}, value)
	if err := ignoreNilError(err); err != nil {
		return 0, err
	}
	return scriptIntResult(res), nil
}
