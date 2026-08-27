package gentity

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newFunctionTestCache(t *testing.T) *RedisCache {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	return NewRedisCache(client)
}

// TestInstallFunctionsAndFCall 验证Function库安装与FCALL路径
// miniredis若不支持FUNCTION(模拟Redis<7.0),验证自动回退EVAL
func TestInstallFunctionsAndFCall(t *testing.T) {
	cache := newFunctionTestCache(t)
	err := cache.InstallFunctions(context.Background())
	if err != nil {
		// 不支持FUNCTION的环境(如miniredis旧版/Redis<7.0):
		// 原子操作应自动回退EVAL,功能不受影响
		t.Logf("InstallFunctions unsupported (fallback to EVAL): %v", err)
	} else {
		t.Logf("functions installed, FCALL enabled")
	}
	// 无论走FCALL还是EVAL,原子操作的行为必须一致
	if _, err := cache.HSetIfAbsentOrEqual("fn:lock", "100", "1"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := cache.HDelIfValueEqual("fn:lock", "100", "2"); ok {
		t.Fatal("value mismatch should not del")
	}
	if ok, _ := cache.HDelIfValueEqual("fn:lock", "100", "1"); !ok {
		t.Fatal("value match should del")
	}
	// 状态检查:安装成功则FCALL可用,否则标记为不可用(回退EVAL)
	state := cache.functionState.Load()
	if err == nil && state != funcStateAvailable {
		t.Fatalf("expected funcStateAvailable, got %v", state)
	}
	if err != nil && state != funcStateUnavailable {
		t.Fatalf("expected funcStateUnavailable, got %v", state)
	}
}
