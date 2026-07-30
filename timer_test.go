package gentity

import (
	"testing"
	"time"
)

// ==================== TimerEntries 测试 ====================

// TestTimerEntries_AddAndRun 验证添加一个过去时间的 timer，Run 时立即执行，job 返回 0 表示不重复
func TestTimerEntries_AddAndRun(t *testing.T) {
	te := NewTimerEntries()
	te.Start()
	defer te.Stop()

	var counter int
	// 添加一个过去时间的 timer，确保 Run 时立即执行
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		counter++
		return 0
	})

	ran := te.Run(time.Now())
	if !ran {
		t.Fatal("Run should return true since a job ran")
	}
	if counter != 1 {
		t.Fatalf("expected counter 1, got %d", counter)
	}
	// entry 已被移除
	if len(te.entries) != 0 {
		t.Fatalf("expected 0 entries after run, got %d", len(te.entries))
	}
}

// TestTimerEntries_After 验证 After 添加的定时器在到期后 Run 能执行回调
func TestTimerEntries_After(t *testing.T) {
	// 用很短的 minInterval 加快测试
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	var counter int
	te.After(time.Millisecond*10, func() time.Duration {
		counter++
		return 0
	})

	// 等待足够时间使 entry.next 过期
	time.Sleep(time.Millisecond * 50)
	te.Run(time.Now())

	if counter != 1 {
		t.Fatalf("expected counter 1, got %d", counter)
	}
}

// TestTimerEntries_Recurring 验证 job 返回 d > 0 时重复执行
func TestTimerEntries_Recurring(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	var counter int
	d := time.Millisecond * 20
	te.After(d, func() time.Duration {
		counter++
		return d
	})

	// 第一次 Run：等待 d 后到期执行
	time.Sleep(d * 3)
	te.Run(time.Now())
	if counter != 1 {
		t.Fatalf("after first run: expected counter 1, got %d", counter)
	}
	if len(te.entries) != 1 {
		t.Fatalf("expected 1 entry remaining (recurring), got %d", len(te.entries))
	}

	// 再等待 d，第二次 Run：job 被再次调用，entry.next 已被更新
	time.Sleep(d * 3)
	te.Run(time.Now())
	if counter != 2 {
		t.Fatalf("after second run: expected counter 2, got %d", counter)
	}
}

// TestTimerEntries_MultipleTimers 验证多个 timer 按时间顺序执行，到期执行未到期不执行
func TestTimerEntries_MultipleTimers(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	now := time.Now()
	var callOrder []int

	// timer1: 已到期（更早）
	te.AddTimer(now.Add(-2*time.Second), func() time.Duration {
		callOrder = append(callOrder, 1)
		return 0
	})
	// timer2: 远未到期
	te.AddTimer(now.Add(time.Hour), func() time.Duration {
		callOrder = append(callOrder, 2)
		return 0
	})
	// timer3: 已到期（比 timer1 晚）
	te.AddTimer(now.Add(-1*time.Second), func() time.Duration {
		callOrder = append(callOrder, 3)
		return 0
	})

	ran := te.Run(now)
	if !ran {
		t.Fatal("Run should return true since some jobs ran")
	}

	// 只有到期的 timer1 和 timer3 被执行，timer2 未到期不执行
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 jobs called, got %d (%v)", len(callOrder), callOrder)
	}
	// 按时间顺序执行：先到期（timer1）后到期（timer3）
	if callOrder[0] != 1 || callOrder[1] != 3 {
		t.Fatalf("expected call order [1 3], got %v", callOrder)
	}
	for _, id := range callOrder {
		if id == 2 {
			t.Fatal("timer2 should not have been called (not expired)")
		}
	}
}

// TestTimerEntries_Now 验证 Now 的默认行为、时间偏移以及自定义 nowFunc
func TestTimerEntries_Now(t *testing.T) {
	// abs 返回 duration 的绝对值
	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return -d
		}
		return d
	}

	te := NewTimerEntries()

	// 默认 Now() 接近 time.Now()（允许小偏差）
	if diff := abs(te.Now().Sub(time.Now())); diff > time.Second {
		t.Fatalf("Now() should be close to time.Now(), diff=%v", diff)
	}

	// SetTimeOffset 后 Now() 接近 time.Now().Add(offset)
	offset := time.Hour
	te.SetTimeOffset(offset)
	if te.GetTimeOffset() != offset {
		t.Fatalf("expected offset %v, got %v", offset, te.GetTimeOffset())
	}
	offsetNow := te.Now()
	expected := time.Now().Add(offset)
	if abs(expected.Sub(offsetNow)) > time.Second {
		t.Fatalf("Now() with offset should be close to time.Now()+offset, diff=%v", abs(expected.Sub(offsetNow)))
	}

	// NewTimerEntriesWithArgs 自定义 nowFunc 时返回自定义时间
	custom := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	te2 := NewTimerEntriesWithArgs(func() time.Time {
		return custom
	}, time.Second)
	if !te2.Now().Equal(custom) {
		t.Fatalf("custom nowFunc should return %v, got %v", custom, te2.Now())
	}
}

// TestTimerEntries_SortOrder 验证 Start 后 entries 按时间升序排序
func TestTimerEntries_SortOrder(t *testing.T) {
	te := NewTimerEntries()

	now := time.Now()
	t1 := now.Add(time.Second)
	t2 := now.Add(2 * time.Second)
	t3 := now.Add(3 * time.Second)

	// 乱序添加：t3, t1, t2
	te.AddTimer(t3, func() time.Duration { return 0 })
	te.AddTimer(t1, func() time.Duration { return 0 })
	te.AddTimer(t2, func() time.Duration { return 0 })

	te.Start()
	defer te.Stop()

	if len(te.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(te.entries))
	}
	if !te.entries[0].next.Equal(t1) {
		t.Errorf("entries[0] should be t1(%v), got %v", t1, te.entries[0].next)
	}
	if !te.entries[1].next.Equal(t2) {
		t.Errorf("entries[1] should be t2(%v), got %v", t2, te.entries[1].next)
	}
	if !te.entries[2].next.Equal(t3) {
		t.Errorf("entries[2] should be t3(%v), got %v", t3, te.entries[2].next)
	}
}

// TestTimerEntries_Stop 验证 Start 后 Stop 不应 panic
func TestTimerEntries_Stop(t *testing.T) {
	te := NewTimerEntries()
	te.Start()
	// Stop 停止 Timer，不应 panic
	te.Stop()
}
