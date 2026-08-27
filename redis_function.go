package gentity

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

// ==================== Redis Function ====================
// Redis 7.0+支持FUNCTION命令,脚本持久化在服务器端(RDB/AOF),具备版本管理能力
// 相比EVAL/EVALSHA的优势:
//   - 脚本随服务器持久化,重启/SCRIPT FLUSH后不丢失,无需每次进程启动重新分发
//   - FUNCTION LIST可审计线上运行的脚本版本,LOAD REPLACE支持热更新
//   - FCALL的错误信息携带函数名,排查问题更直观
//
// 注意:Redis Cluster中FUNCTION不会在节点间自动同步,需向所有master节点安装(见InstallFunctions)
// 兼容性设计:FCALL失败(版本低于7.0或库未安装)时自动回退到等价的EVAL脚本,功能完全一致,
// 因此InstallFunctions是可选的增强操作,未调用也不影响任何功能

// 函数库源码
// 所有函数均只操作KEYS[1]单个key,天然满足Redis Cluster"所有key在同一slot"的限制
// 函数体与redis_cache.go中的EVAL回退脚本保持一致,修改时需同步修改两处
const redisFunctionLibrarySource = `#!lua name=gentity

local function replace_map(keys, args)
	redis.call('DEL', keys[1])
	local n = #args
	local i = 1
	while i <= n do
		redis.call('HSET', keys[1], args[i], args[i+1])
		i = i + 2
	end
	return 1
end

local function update_map(keys, args)
	local n = tonumber(args[1])
	for i = 1, n do
		redis.call('HSET', keys[1], args[2*i], args[2*i+1])
	end
	for i = 2*n+2, #args do
		redis.call('HDEL', keys[1], args[i])
	end
	return 1
end

local function hset_if_absent_or_equal(keys, args)
	local cur = redis.call('HGET', keys[1], args[1])
	if cur == false or cur == args[2] then
		redis.call('HSET', keys[1], args[1], args[2])
		return 1
	end
	return 0
end

local function hdel_if_value_equal(keys, args)
	if redis.call('HGET', keys[1], args[1]) == args[2] then
		return redis.call('HDEL', keys[1], args[1])
	end
	return 0
end

local function hdel_fields_by_value(keys, args)
	local all = redis.call('HGETALL', keys[1])
	local n = 0
	for i = 1, #all, 2 do
		if all[i+1] == args[1] then
			redis.call('HDEL', keys[1], all[i])
			n = n + 1
		end
	end
	return n
end

-- NOTE:Redis 7.x实测,函数名/库名只允许字母/数字/下划线,
-- 含'.'或'-'的名字会导致FUNCTION LOAD失败(ERR Library names can only contain...),
-- 因此函数名用下划线而非"库名.函数名"的点号风格
redis.register_function('gentity_replace_map', replace_map)
redis.register_function('gentity_update_map', update_map)
redis.register_function('gentity_hset_if_absent_or_equal', hset_if_absent_or_equal)
redis.register_function('gentity_hdel_if_value_equal', hdel_if_value_equal)
redis.register_function('gentity_hdel_fields_by_value', hdel_fields_by_value)
`

// 函数库中的函数名
const (
	funcReplaceMap          = "gentity_replace_map"
	funcUpdateMap           = "gentity_update_map"
	funcHSetIfAbsentOrEqual = "gentity_hset_if_absent_or_equal"
	funcHDelIfValueEqual    = "gentity_hdel_if_value_equal"
	funcHDelFieldsByValue   = "gentity_hdel_fields_by_value"
)

// Function不可用时回退到等价的EVAL脚本
var functionEvalScripts = map[string]*redis.Script{
	funcReplaceMap:          luaReplaceMap,
	funcUpdateMap:           luaUpdateMap,
	funcHSetIfAbsentOrEqual: luaHSetIfAbsentOrEqual,
	funcHDelIfValueEqual:    luaHDelIfValueEqual,
	funcHDelFieldsByValue:   luaHDelFieldsByValue,
}

// Function库可用状态(FunctionRunner.state使用)
const (
	funcStateUnknown     = int32(0) // 未探测
	funcStateAvailable   = int32(1) // FCALL可用
	funcStateUnavailable = int32(2) // 不可用(版本低于7.0或库未安装),回退EVAL
)

// isFunctionsUnavailableError 判断错误是否表示"Function能力不可用"
// - Redis < 7.0: ERR unknown command 'FCALL'
// - 库未安装或函数不存在(Redis 7.x实测): ERR Function not found
// - 兼容其他变体: ERR No function named / No function library
// 其余错误(如集群跨slot)是真实的调用错误,应返回给调用方而不是静默回退
func isFunctionsUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "function not found") ||
		strings.Contains(msg, "no function named") ||
		strings.Contains(msg, "no function library")
}

// FunctionRunner Redis Function执行器:优先FCALL,不可用(Redis<7.0或库未安装)自动回退EVAL脚本
// 供应用层注册自己的原子函数库使用(gentity内部也用它执行gentity库):
//
//	runner := gentity.NewFunctionRunner(client, map[string]*redis.Script{
//	    "my_func": redis.NewScript(`...lua...`), // FCALL不可用时的等价回退脚本
//	})
//	gentity.InstallRedisFunctionLibrary(ctx, client, myLibrarySource) // 可选,安装后走FCALL
//	runner.Run("my_func", []string{key}, args...)
//
// 且回退对调用方完全透明:两条路径的函数体必须等价,keys/args协议一致
type FunctionRunner struct {
	client      redis.Cmdable
	state       atomic.Int32
	evalScripts map[string]*redis.Script
}

// NewFunctionRunner 创建函数执行器
// evalScripts:函数名到EVAL回退脚本的映射
func NewFunctionRunner(client redis.Cmdable, evalScripts map[string]*redis.Script) *FunctionRunner {
	return &FunctionRunner{
		client:      client,
		evalScripts: evalScripts,
	}
}

// Run 执行原子函数:优先FCALL,不可用时回退evalScripts[name]的EVAL脚本
// keys/args协议与EVAL完全一致
func (this *FunctionRunner) Run(name string, keys []string, args ...interface{}) (interface{}, error) {
	if this.state.Load() != funcStateUnavailable {
		res, err := this.client.FCall(context.Background(), name, keys, args...).Result()
		if err == nil {
			this.state.Store(funcStateAvailable)
			return res, nil
		}
		if isFunctionsUnavailableError(err) {
			// Function不可用,此后直接走EVAL,不再重复尝试FCALL
			this.state.Store(funcStateUnavailable)
		} else {
			return nil, err
		}
	}
	script, ok := this.evalScripts[name]
	if !ok {
		return nil, fmt.Errorf("FunctionRunner.Run: unknown function %v", name)
	}
	return script.Run(context.Background(), this.client, keys, args...).Result()
}

// MarkAvailable 标记函数库已安装可用
// 库安装成功后调用,使此前因库缺失降级为EVAL的执行器恢复FCALL路径
func (this *FunctionRunner) MarkAvailable() {
	this.state.Store(funcStateAvailable)
}

// Available 当前是否走FCALL路径(库已安装且探测成功)
func (this *FunctionRunner) Available() bool {
	return this.state.Load() == funcStateAvailable
}

// InstallRedisFunctionLibrary 安装Redis Function库(FUNCTION LOAD REPLACE,幂等,可用于热更新)
// 集群模式:FUNCTION不会在节点间自动同步,会安装到所有master节点
// 单机/主从模式:安装到当前连接的节点
// 库名/函数名只能包含字母/数字/下划线(Redis 7.x限制)
func InstallRedisFunctionLibrary(ctx context.Context, client redis.Cmdable, librarySource string) error {
	loadFn := func(ctx context.Context, client *redis.Client) error {
		return client.FunctionLoadReplace(ctx, librarySource).Err()
	}
	switch c := client.(type) {
	case *redis.Client:
		return loadFn(ctx, c)
	case *redis.ClusterClient:
		return c.ForEachMaster(ctx, loadFn)
	default:
		return fmt.Errorf("InstallRedisFunctionLibrary: unsupported redis client type %T", client)
	}
}

// runFunction 执行原子操作(委托给FunctionRunner)
// keys/args协议与EVAL完全一致
func (this *RedisCache) runFunction(name string, keys []string, args ...interface{}) (interface{}, error) {
	return this.funcRunner.Run(name, keys, args...)
}

// InstallFunctions 安装gentity的Redis Function库(幂等,REPLACE语义,可用于脚本热更新)
// 集群模式:FUNCTION不会在节点间自动同步,会安装到所有master节点
// 单机/主从模式:安装到当前连接的节点
// 安装后,原子操作自动从EVAL切换为FCALL;未安装或Redis版本低于7.0时,自动回退EVAL,功能不受影响
// 建议在应用初始化(如连接Redis后)时调用一次
func (this *RedisCache) InstallFunctions(ctx context.Context) error {
	err := InstallRedisFunctionLibrary(ctx, this.redisClient, redisFunctionLibrarySource)
	if err == nil {
		// 库已就绪,重置探测状态:此前因库缺失降级为EVAL的实例,恢复FCALL路径
		this.funcRunner.MarkAvailable()
	}
	return err
}
