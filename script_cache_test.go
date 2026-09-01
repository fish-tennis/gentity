package gentity

import (
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// 使用miniredis验证ScriptKvCache的Lua脚本行为
// miniredis支持EVAL/EVALSHA,可在无真实Redis的环境下验证脚本语义
func newScriptTestCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	return NewRedisCache(client), server
}

func TestAtomicReplaceMap(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:replace"
	// 预置旧数据,验证replace会清除旧field
	if err := cache.SetMap(key, map[string]interface{}{"old": "1", "keep": "2"}); err != nil {
		t.Fatal(err)
	}
	if err := cache.AtomicReplaceMap(key, map[string]interface{}{"new": "3"}); err != nil {
		t.Fatal(err)
	}
	m, err := cache.HGetAll(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m["new"] != "3" {
		t.Fatalf("AtomicReplaceMap err: %v", m)
	}
	// 空map等价于删除
	if err := cache.AtomicReplaceMap(key, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.HGetAll(key); err != nil {
		t.Fatal(err)
	}
	cacheType, _ := cache.Type(key)
	if cacheType != "none" {
		t.Fatalf("empty map should del key, type=%v", cacheType)
	}
}

func TestAtomicUpdateMap(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:update"
	// 预置数据
	if err := cache.SetMap(key, map[string]interface{}{"a": "1", "b": "2", "c": "3"}); err != nil {
		t.Fatal(err)
	}
	// set和del同时原子执行
	if err := cache.AtomicUpdateMap(key,
		map[string]interface{}{"b": "22", "d": "4"},
		[]string{"a", "c"}); err != nil {
		t.Fatal(err)
	}
	m, err := cache.HGetAll(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["b"] != "22" || m["d"] != "4" {
		t.Fatalf("AtomicUpdateMap err: %v", m)
	}
	// 只del
	if err := cache.AtomicUpdateMap(key, nil, []string{"d"}); err != nil {
		t.Fatal(err)
	}
	m, _ = cache.HGetAll(key)
	if len(m) != 1 || m["b"] != "22" {
		t.Fatalf("AtomicUpdateMap del-only err: %v", m)
	}
	// 只set
	if err := cache.AtomicUpdateMap(key, map[string]interface{}{"e": "5"}, nil); err != nil {
		t.Fatal(err)
	}
	m, _ = cache.HGetAll(key)
	if len(m) != 2 || m["e"] != "5" {
		t.Fatalf("AtomicUpdateMap set-only err: %v", m)
	}
	// 两者皆空:无操作
	if err := cache.AtomicUpdateMap(key, nil, nil); err != nil {
		t.Fatal(err)
	}
	m, _ = cache.HGetAll(key)
	if len(m) != 2 {
		t.Fatalf("AtomicUpdateMap no-op err: %v", m)
	}
	// 非map[string]类型:setMap支持map[interface{}]interface{}(SaveMapValueToCache的增量格式)
	if err := cache.AtomicUpdateMap(key,
		map[interface{}]interface{}{"f": "6"}, nil); err != nil {
		t.Fatal(err)
	}
	m, _ = cache.HGetAll(key)
	if len(m) != 3 || m["f"] != "6" {
		t.Fatalf("AtomicUpdateMap interface map err: %v", m)
	}
}

func TestHSetIfAbsentOrEqual(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:lock"
	// 无锁:加锁成功
	ok, err := cache.HSetIfAbsentOrEqual(key, "100", "1")
	if err != nil || !ok {
		t.Fatalf("lock empty err: ok=%v err=%v", ok, err)
	}
	// 他服持有:加锁失败
	ok, err = cache.HSetIfAbsentOrEqual(key, "100", "2")
	if err != nil || ok {
		t.Fatalf("lock other err: ok=%v err=%v", ok, err)
	}
	// 本服持有(崩溃残留):可重入成功
	ok, err = cache.HSetIfAbsentOrEqual(key, "100", "1")
	if err != nil || !ok {
		t.Fatalf("lock reentrant err: ok=%v err=%v", ok, err)
	}
	// 不同field互不影响
	ok, err = cache.HSetIfAbsentOrEqual(key, "200", "2")
	if err != nil || !ok {
		t.Fatalf("lock another field err: ok=%v err=%v", ok, err)
	}
}

func TestHDelIfValueEqual(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:unlock"
	if _, err := cache.HSet(key, "100", "1"); err != nil {
		t.Fatal(err)
	}
	// 值不匹配:不删除
	ok, err := cache.HDelIfValueEqual(key, "100", "2")
	if err != nil || ok {
		t.Fatalf("unlock value mismatch err: ok=%v err=%v", ok, err)
	}
	m, _ := cache.HGetAll(key)
	if len(m) != 1 {
		t.Fatalf("mismatch should not del: %v", m)
	}
	// 值匹配:删除成功
	ok, err = cache.HDelIfValueEqual(key, "100", "1")
	if err != nil || !ok {
		t.Fatalf("unlock value match err: ok=%v err=%v", ok, err)
	}
	m, _ = cache.HGetAll(key)
	if len(m) != 0 {
		t.Fatalf("match should del: %v", m)
	}
	// field不存在:返回false
	ok, err = cache.HDelIfValueEqual(key, "100", "1")
	if err != nil || ok {
		t.Fatalf("unlock not exist err: ok=%v err=%v", ok, err)
	}
}

func TestHDelFieldsByValue(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:cleanup"
	if err := cache.SetMap(key, map[string]interface{}{
		"100": "1", "101": "1", "102": "2",
	}); err != nil {
		t.Fatal(err)
	}
	// 只删除值等于"1"的field,值等于"2"的保留
	deleted, err := cache.HDelFieldsByValue(key, "1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%v", deleted)
	}
	m, _ := cache.HGetAll(key)
	if len(m) != 1 || m["102"] != "2" {
		t.Fatalf("HDelFieldsByValue err: %v", m)
	}
	// 无匹配:删除0个
	deleted, err = cache.HDelFieldsByValue(key, "3")
	if err != nil || deleted != 0 {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
}

// 验证HDelFieldsByValue分批HDEL逻辑:field数超过单批上限(500)时跨批次正确删除
// 覆盖lua脚本中math.min(i+499, n)的批次边界
func TestHDelFieldsByValueBatch(t *testing.T) {
	cache, _ := newScriptTestCache(t)
	key := "test:cleanup_batch"
	const total = 1200 // 500+500+200,跨3批
	fields := make(map[string]interface{}, total)
	for i := 0; i < total; i++ {
		fields[strconv.Itoa(i)] = "1"
	}
	if err := cache.SetMap(key, fields); err != nil {
		t.Fatal(err)
	}
	// 混入其他值的field,验证不会被误删
	if _, err := cache.HSet(key, "other", "2"); err != nil {
		t.Fatal(err)
	}
	deleted, err := cache.HDelFieldsByValue(key, "1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != total {
		t.Fatalf("deleted=%v want=%v", deleted, total)
	}
	m, _ := cache.HGetAll(key)
	if len(m) != 1 || m["other"] != "2" {
		t.Fatalf("HDelFieldsByValue batch err: %v", m)
	}
	// 第二批之后剩余0个匹配,验证跨批次循环终止
	if deleted, err = cache.HDelFieldsByValue(key, "1"); err != nil || deleted != 0 {
		t.Fatalf("second call deleted=%v err=%v", deleted, err)
	}
}
