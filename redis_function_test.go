package gentity

import (
	"context"
	"fmt"
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
	state := cache.funcRunner.state.Load()
	if err == nil && state != funcStateAvailable {
		t.Fatalf("expected funcStateAvailable, got %v", state)
	}
	if err != nil && state != funcStateUnavailable {
		t.Fatalf("expected funcStateUnavailable, got %v", state)
	}
}

// TestIsFunctionsUnavailableError 验证Function不可用错误的判定
// 命中→自动降级EVAL;未命中→错误原样返回给调用方,判定面过宽/过窄都有真实后果
func TestIsFunctionsUnavailableError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		// 命中:Redis<7.0
		{"redis_quote", fmt.Errorf("ERR unknown command 'FCALL'"), true},
		// 命中:miniredis格式
		{"miniredis_backquote", fmt.Errorf("ERR unknown command `fcall`, with args beginning with: "), true},
		// 命中:无引号变体
		{"no_quote", fmt.Errorf("ERR unknown command fcall"), true},
		// 命中:部分兼容实现/云Redis的带冒号格式
		{"colon_quote", fmt.Errorf("ERR unknown command: 'FCALL'"), true},
		// 命中:库未安装/函数不存在(Redis 7.x实测文案)
		{"function_not_found", fmt.Errorf("ERR Function not found"), true},
		{"no_function_named", fmt.Errorf("ERR no function named"), true},
		{"no_function_library", fmt.Errorf("ERR no function library"), true},
		// 不命中:其他命令的unknown command,不能误触发永久降级
		{"other_command", fmt.Errorf("ERR unknown command 'GET'"), false},
		// 不命中:其他命令的unknown command,即使文案后段出现fcall字样
		{"other_command_with_fcall_mention", fmt.Errorf("ERR unknown command 'MY_FUNC', use FCALL instead"), false},
		// 不命中:文案其他位置出现fcall字样
		{"fcall_in_middle", fmt.Errorf("ERR fcall is not allowed in this context"), false},
		// 不命中:真实调用错误(如集群跨slot),应返回给调用方
		{"cluster_cross_slot", fmt.Errorf("ERR CROSSSLOT Keys in request don't hash to the same slot"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := isFunctionsUnavailableError(c.err); got != c.want {
			t.Errorf("%v: isFunctionsUnavailableError(%v)=%v, want %v", c.name, c.err, got, c.want)
		}
	}
}
