package gentity

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"
)

// ==================== 真实Redis集成测试 ====================
// 依赖本地Redis实例(默认127.0.0.1:6379),连接失败时Skip
// 与miniredis测试互补:真实Redis支持FUNCTION/FCALL,可验证FCALL路径与回退行为
//
// 使用独立的 key 前缀 gtest: 和锁名 gtest:dlock,测试结束清理,不污染其他数据

const testRedisAddr = "127.0.0.1:6379"

func newRealRedisCache(t *testing.T) *RedisCache {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: testRedisAddr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Skipf("local redis %v unavailable, skip integration test: %v", testRedisAddr, err)
	}
	t.Cleanup(func() {
		// 恢复环境:删除测试key与函数库(下次运行会重新安装)
		client.Del(context.Background(),
			"gtest:map", "gtest:upd", "gtest:lock", "gtest:conc", "gtest:dlock",
			"gtest:replace", "gtest:cons:a", "gtest:cons:b")
		client.FunctionDelete(context.Background(), "gentity")
		client.Close()
	})
	return NewRedisCache(client)
}

// requireFunctionsInstalled 安装Function库;Redis<7.0不支持时Skip FCALL相关断言
func requireFunctionsInstalled(t *testing.T, cache *RedisCache) bool {
	t.Helper()
	if err := cache.InstallFunctions(context.Background()); err != nil {
		t.Skipf("redis functions unsupported (redis<7.0?), skip FCALL test: %v", err)
	}
	return true
}

// TestRealRedis_InstallAndFCall 安装后所有原子操作走FCALL且行为正确
func TestRealRedis_InstallAndFCall(t *testing.T) {
	cache := newRealRedisCache(t)
	requireFunctionsInstalled(t, cache)

	// AtomicReplaceMap:替换并清除旧field
	cache.SetMap("gtest:replace", map[string]interface{}{"old": "1"})
	if err := cache.AtomicReplaceMap("gtest:replace", map[string]interface{}{"a": "1", "b": "2"}); err != nil {
		t.Fatal(err)
	}
	m, _ := cache.HGetAll("gtest:replace")
	if len(m) != 2 || m["a"] != "1" || m["b"] != "2" {
		t.Fatalf("AtomicReplaceMap: %v", m)
	}

	// AtomicUpdateMap:set+del混合
	if err := cache.AtomicUpdateMap("gtest:upd",
		map[string]interface{}{"b": "22", "c": "3"}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	m, _ = cache.HGetAll("gtest:upd")
	if len(m) != 2 || m["b"] != "22" || m["c"] != "3" {
		t.Fatalf("AtomicUpdateMap: %v", m)
	}

	// HSetIfAbsentOrEqual:不存在/同值/他值
	if ok, _ := cache.HSetIfAbsentOrEqual("gtest:lock", "100", "101"); !ok {
		t.Fatal("absent should set")
	}
	if ok, _ := cache.HSetIfAbsentOrEqual("gtest:lock", "100", "101"); !ok {
		t.Fatal("same owner should reentrant")
	}
	if ok, _ := cache.HSetIfAbsentOrEqual("gtest:lock", "100", "102"); ok {
		t.Fatal("other owner should fail")
	}

	// HDelIfValueEqual
	if ok, _ := cache.HDelIfValueEqual("gtest:lock", "100", "102"); ok {
		t.Fatal("mismatch should not del")
	}
	if ok, _ := cache.HDelIfValueEqual("gtest:lock", "100", "101"); !ok {
		t.Fatal("match should del")
	}

	// HDelFieldsByValue
	cache.SetMap("gtest:lock", map[string]interface{}{"1": "101", "2": "101", "3": "888"})
	if n, _ := cache.HDelFieldsByValue("gtest:lock", "101"); n != 2 {
		t.Fatalf("HDelFieldsByValue: n=%v", n)
	}
	m, _ = cache.HGetAll("gtest:lock")
	if len(m) != 1 || m["3"] != "888" {
		t.Fatalf("HDelFieldsByValue residue: %v", m)
	}

	// 全部操作走的是FCALL路径
	if cache.funcRunner.state.Load() != funcStateAvailable {
		t.Fatalf("expected FCALL state, got %v", cache.funcRunner.state.Load())
	}
}

// TestRealRedis_FallbackOnFunctionDelete 验证库被删除后自动回退EVAL
// 回归场景:FCALL返回"ERR Function not found"(Redis 7.x实测文本),
// 必须触发回退而不是把错误抛给调用方
func TestRealRedis_FallbackOnFunctionDelete(t *testing.T) {
	cache := newRealRedisCache(t)
	requireFunctionsInstalled(t, cache)
	client := cache.redisClient.(*redis.Client)

	// 先确认FCALL可用
	if _, err := cache.HSetIfAbsentOrEqual("gtest:lock", "1", "1"); err != nil {
		t.Fatal(err)
	}
	if cache.funcRunner.state.Load() != funcStateAvailable {
		t.Fatal("expected available before delete")
	}
	// 模拟库丢失:运维执行了FUNCTION FLUSH/FUNCTION DELETE/数据库故障切换等
	if err := client.FunctionDelete(context.Background(), "gentity").Err(); err != nil {
		t.Fatalf("FunctionDelete: %v", err)
	}
	// 下一次原子操作应自动回退EVAL且行为正确(而非返回Function not found错误)
	if ok, err := cache.HSetIfAbsentOrEqual("gtest:lock", "2", "2"); err != nil || !ok {
		t.Fatalf("after library deleted, should fallback to EVAL: ok=%v err=%v", ok, err)
	}
	if cache.funcRunner.state.Load() != funcStateUnavailable {
		t.Fatalf("expected unavailable after fallback, got %v", cache.funcRunner.state.Load())
	}
	// EVAL路径下其余方法仍正常
	if ok, _ := cache.HDelIfValueEqual("gtest:lock", "2", "2"); !ok {
		t.Fatal("EVAL path HDelIfValueEqual failed")
	}
	if err := cache.AtomicReplaceMap("gtest:replace", map[string]interface{}{"x": "1"}); err != nil {
		t.Fatalf("EVAL path AtomicReplaceMap: %v", err)
	}
}

// TestRealRedis_ReinstallRestoresFCALL 验证重新安装后恢复FCALL路径
func TestRealRedis_ReinstallRestoresFCall(t *testing.T) {
	cache := newRealRedisCache(t)
	requireFunctionsInstalled(t, cache)
	client := cache.redisClient.(*redis.Client)

	// 制造降级
	client.FunctionDelete(context.Background(), "gentity")
	if _, err := cache.HSetIfAbsentOrEqual("gtest:lock", "1", "1"); err != nil {
		t.Fatal(err)
	}
	if cache.funcRunner.state.Load() != funcStateUnavailable {
		t.Fatal("expected unavailable")
	}
	// 重新安装成功后,应恢复FCALL
	if err := cache.InstallFunctions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.funcRunner.state.Load() != funcStateAvailable {
		t.Fatalf("expected available after reinstall, got %v", cache.funcRunner.state.Load())
	}
	if ok, err := cache.HSetIfAbsentOrEqual("gtest:lock", "3", "3"); err != nil || !ok {
		t.Fatalf("FCALL after reinstall: ok=%v err=%v", ok, err)
	}
}

// ==================== 分布式锁集成 ====================

type testIntegrationApp struct {
	wg sync.WaitGroup
}

func (a *testIntegrationApp) GetId() int32                      { return 101 }
func (a *testIntegrationApp) GetContext() context.Context       { return context.Background() }
func (a *testIntegrationApp) GetWaitGroup() *sync.WaitGroup     { return &a.wg }
func (a *testIntegrationApp) Init(context.Context, string) bool { return true }
func (a *testIntegrationApp) Run(context.Context)               {}
func (a *testIntegrationApp) OnUpdate(context.Context, int64)   {}
func (a *testIntegrationApp) Exit()                             {}

// TestRealRedis_DistributedLock 分布式锁全流程(走脚本原子路径)
func TestRealRedis_DistributedLock(t *testing.T) {
	cache := newRealRedisCache(t)
	prevApp := GetApplication()
	SetApplication(&testIntegrationApp{})
	defer SetApplication(prevApp)

	mgr := NewDistributedEntityMgr("gtest:dlock", nil, cache, nil, nil)

	// 加锁+同服重入(服务器重启后重新占有自己的残留锁)
	if !mgr.DistributeLock(1) {
		t.Fatal("lock should succeed")
	}
	if !mgr.DistributeLock(1) {
		t.Fatal("reentrant lock should succeed")
	}
	// 他服持有的实体:加锁失败
	cache.redisClient.HSet(context.Background(), "gtest:dlock", "2", "888")
	if mgr.DistributeLock(2) {
		t.Fatal("lock other's entity should fail")
	}
	// 解锁:仅删自己的
	mgr.DistributeUnlock(1)
	kv, _ := cache.HGetAll("gtest:dlock")
	if len(kv) != 1 || kv["2"] != "888" {
		t.Fatalf("after unlock: %v", kv)
	}
	// DeleteDistributeLocks:只清理本服(101)的锁,他服(888)的保留
	cache.redisClient.HSet(context.Background(), "gtest:dlock", "3", "101")
	cache.redisClient.HSet(context.Background(), "gtest:dlock", "4", "101")
	mgr.DeleteDistributeLocks()
	kv, _ = cache.HGetAll("gtest:dlock")
	if len(kv) != 1 || kv["2"] != "888" {
		t.Fatalf("after DeleteDistributeLocks: %v", kv)
	}
}

// TestRealRedis_ConcurrentLockRace 并发抢锁:同一field恰好一个serverId占有
// 并发调用底层的HSetIfAbsentOrEqual模拟多台服务器(避免并发SetApplication的数据竞争)
func TestRealRedis_ConcurrentLockRace(t *testing.T) {
	cache := newRealRedisCache(t)

	const goroutines = 16
	var successCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := cache.HSetIfAbsentOrEqual("gtest:dlock", "777", fmt.Sprintf("%v", 300+idx))
			if err != nil {
				t.Errorf("g%v: %v", idx, err)
				return
			}
			if ok {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if n := successCount.Load(); n != 1 {
		t.Fatalf("exactly one server should win the lock, got %v", n)
	}
	// 值必须是胜出者之一写入的完整值
	v, err := cache.redisClient.(*redis.Client).HGet(context.Background(), "gtest:dlock", "777").Result()
	if err != nil || len(v) != 3 || v[0] != '3' {
		t.Fatalf("lock value corrupted: %q err=%v", v, err)
	}
}

// ==================== save路径集成 ====================

// TestRealRedis_SaveMapToCache MapData保存缓存:首次全量SetMap + 增量AtomicUpdateMap
func TestRealRedis_SaveMapToCache(t *testing.T) {
	cache := newRealRedisCache(t)

	obj := NewMapData[int32, int32]()
	obj.Set(1, 10)
	obj.Set(2, 20)
	objStruct := GetObjSaveableStruct(obj)
	fieldObj, field := objStruct.GetSingleSaveable(obj)

	// 首次保存:全量(SetMap)
	SaveChangedDataToCache(cache, fieldObj, "gtest:map", field)
	m, _ := cache.HGetAll("gtest:map")
	if len(m) != 2 || m["1"] != "10" || m["2"] != "20" {
		t.Fatalf("first save: %v", m)
	}
	if !obj.HasCached() {
		t.Fatal("HasCached should be true")
	}

	// 增量:Set新值+Delete旧值,单脚本原子应用(AtomicUpdateMap)
	obj.Set(3, 30)
	obj.Delete(1)
	SaveChangedDataToCache(cache, fieldObj, "gtest:map", field)
	m, _ = cache.HGetAll("gtest:map")
	if len(m) != 2 || m["2"] != "20" || m["3"] != "30" {
		t.Fatalf("incremental save: %v", m)
	}
	if _, ok := m["1"]; ok {
		t.Fatalf("deleted key 1 should be removed: %v", m)
	}
}

// TestRealRedis_EvalFcallConsistency EVAL与FCALL双路径结果一致
func TestRealRedis_EvalFcallConsistency(t *testing.T) {
	cache := newRealRedisCache(t)
	requireFunctionsInstalled(t, cache)

	// 强制EVAL路径的实例
	evalCache := NewRedisCache(cache.redisClient)
	evalCache.funcRunner.state.Store(funcStateUnavailable)

	keyA, keyB := "gtest:cons:a", "gtest:cons:b"
	steps := []func(c ScriptKvCache, key string) error{
		func(c ScriptKvCache, key string) error {
			return c.AtomicReplaceMap(key, map[string]interface{}{"a": "1", "b": "2", "c": "3"})
		},
		func(c ScriptKvCache, key string) error {
			return c.AtomicUpdateMap(key, map[string]interface{}{"b": "22", "d": "4"}, []string{"a", "c"})
		},
		func(c ScriptKvCache, key string) error {
			_, err := c.HSetIfAbsentOrEqual(key, "f", "9")
			return err
		},
		func(c ScriptKvCache, key string) error {
			_, err := c.HDelIfValueEqual(key, "d", "4")
			return err
		},
	}
	for _, step := range steps {
		if err := step(cache, keyA); err != nil {
			t.Fatalf("FCALL path: %v", err)
		}
		if err := step(evalCache, keyB); err != nil {
			t.Fatalf("EVAL path: %v", err)
		}
	}
	ma, _ := cache.HGetAll(keyA)
	mb, _ := cache.HGetAll(keyB)
	if len(ma) != len(mb) {
		t.Fatalf("divergence: %v vs %v", ma, mb)
	}
	for k, v := range ma {
		if mb[k] != v {
			t.Fatalf("divergence at %v: %v vs %v", k, v, mb[k])
		}
	}
	// 路径断言
	if cache.funcRunner.state.Load() != funcStateAvailable {
		t.Fatal("expected FCALL for cache A")
	}
	if evalCache.funcRunner.state.Load() != funcStateUnavailable {
		t.Fatal("expected EVAL for cache B")
	}
}

// TestRealRedis_ConcurrentAtomicUpdate 并发AtomicUpdateMap无错乱
// 多协程交替set/del不同field,最终各field状态必须与串行语义一致(每次操作原子)
func TestRealRedis_ConcurrentAtomicUpdate(t *testing.T) {
	cache := newRealRedisCache(t)

	const goroutines = 8
	const iterations = 100
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				k := fmt.Sprintf("f%v", i%10)
				if i%3 == 0 {
					// 删除
					if err := cache.AtomicUpdateMap("gtest:conc", nil, []string{k}); err != nil {
						errCh <- fmt.Errorf("g%v del: %w", idx, err)
						return
					}
				} else {
					// 写入
					if err := cache.AtomicUpdateMap("gtest:conc",
						map[string]interface{}{k: fmt.Sprintf("v%v-%v", idx, i)}, nil); err != nil {
						errCh <- fmt.Errorf("g%v set: %w", idx, err)
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	// 最终状态:所有值必须是某次完整写入的结果(格式合法),不能出现半成品
	m, _ := cache.HGetAll("gtest:conc")
	for k, v := range m {
		if len(v) < 4 || v[0] != 'v' {
			t.Fatalf("corrupted value at %v: %q", k, v)
		}
	}
}

// TestRealRedis_LuaFunctionListVerifiesInstalled 安装后FUNCTION LIST可见6个函数中的5个
// (同时验证库的Lua源码无语法错误,能在真实Redis上编译注册)
func TestRealRedis_LuaFunctionListVerifiesInstalled(t *testing.T) {
	cache := newRealRedisCache(t)
	requireFunctionsInstalled(t, cache)
	client := cache.redisClient.(*redis.Client)

	list, err := client.FunctionList(context.Background(), redis.FunctionListQuery{LibraryNamePattern: "gentity"}).Result()
	if err != nil || len(list) != 1 {
		t.Fatalf("FunctionList: n=%d err=%v", len(list), err)
	}
	if len(list[0].Functions) != 5 {
		t.Fatalf("expected 5 functions, got %v", len(list[0].Functions))
	}
}
