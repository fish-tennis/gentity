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

	ran := te.Run()
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
	te.Run()

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
	te.Run()
	if counter != 1 {
		t.Fatalf("after first run: expected counter 1, got %d", counter)
	}
	if len(te.entries) != 1 {
		t.Fatalf("expected 1 entry remaining (recurring), got %d", len(te.entries))
	}

	// 再等待 d，第二次 Run：job 被再次调用，entry.next 已被更新
	time.Sleep(d * 3)
	te.Run()
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

	ran := te.Run()
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

// ==================== 修复验证用例 ====================

// TestTimerEntries_BinaryInsertKeepsOrder 验证#6:乱序添加时二分插入保持entries按next升序
func TestTimerEntries_BinaryInsertKeepsOrder(t *testing.T) {
	te := NewTimerEntries()
	now := time.Now()

	// 乱序添加:now+5s, now-1s, now+10s, now+3s, now
	te.AddTimer(now.Add(5*time.Second), func() time.Duration { return 0 })
	te.AddTimer(now.Add(-1*time.Second), func() time.Duration { return 0 })
	te.AddTimer(now.Add(10*time.Second), func() time.Duration { return 0 })
	te.AddTimer(now.Add(3*time.Second), func() time.Duration { return 0 })
	te.AddTimer(now, func() time.Duration { return 0 })

	want := []time.Duration{-1 * time.Second, 0, 3 * time.Second, 5 * time.Second, 10 * time.Second}
	if len(te.entries) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(te.entries))
	}
	for i, d := range want {
		if !te.entries[i].next.Equal(now.Add(d)) {
			t.Errorf("entries[%d] should be now%+v, got %v", i, d, te.entries[i].next.Sub(now))
		}
	}
}

// TestTimerEntries_SameNextTimeFIFO 验证#6附带语义:相同到期时间的timer按添加顺序(FIFO)执行
func TestTimerEntries_SameNextTimeFIFO(t *testing.T) {
	te := NewTimerEntries()
	now := time.Now()
	var order []int

	for i := 1; i <= 3; i++ {
		idx := i
		te.AddTimer(now.Add(-time.Second), func() time.Duration {
			order = append(order, idx)
			return 0
		})
	}

	te.Run()
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected FIFO order [1 2 3], got %v", order)
	}
}

// TestTimerEntries_ZeroTimeEntryNotRemoved 验证#1:Run期间job中添加的零值时间entry不被误删
// 旧实现用next零值做删除标记,该entry会在Run结束的删除循环中被误删
func TestTimerEntries_ZeroTimeEntryNotRemoved(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	var zeroRan bool
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		// Run期间添加零值时间的timer,语义上是"下次Run立即执行"
		te.AddTimer(time.Time{}, func() time.Duration {
			zeroRan = true
			return 0
		})
		return 0
	})

	te.Run()

	// 第一次Run结束后,零值entry不应被误删
	if len(te.entries) != 1 {
		t.Fatalf("zero-time entry should survive first Run, got %d entries", len(te.entries))
	}

	// 第二次Run:零值时间视为已到期,应被执行
	te.Run()
	if !zeroRan {
		t.Fatal("zero-time entry should execute in second Run")
	}
	if len(te.entries) != 0 {
		t.Fatalf("expected 0 entries after second Run, got %d", len(te.entries))
	}
}

// TestTimerEntries_JobPanicRecoversRunning 验证#2:job panic后running标志被defer恢复,定时器系统仍可用
// 注:panic发生在removed标记之前,该entry不会被Run清理,下次Run会再次调用job
// (生产环境routine_entity的外层recover会终结协程,不影响;此处用只panic一次的job适配该语义)
func TestTimerEntries_JobPanicRecoversRunning(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	var afterPanicRan bool
	var panicked bool
	te.AddTimer(time.Now().Add(-2*time.Second), func() time.Duration {
		if !panicked {
			panicked = true
			panic("job panic test")
		}
		// 第二次Run再次调用时正常返回,完成该entry的清理
		return 0
	})

	func() {
		defer func() {
			if err := recover(); err == nil {
				t.Fatal("expected panic from job")
			}
		}()
		te.Run()
	}()

	// panic后running应已被defer恢复为false
	if te.running {
		t.Fatal("running should be false after job panic")
	}
	// panic后新注册的timer应走正常插入路径并触发
	te.AddTimer(time.Now().Add(-1*time.Second), func() time.Duration {
		afterPanicRan = true
		return 0
	})
	te.Run()
	if !afterPanicRan {
		t.Fatal("timer added after job panic should execute normally")
	}
}

// TestTimerEntries_BeforeStartNoPanic 验证#3:Start之前AddTimer/Run不会因Timer为nil而panic
func TestTimerEntries_BeforeStartNoPanic(t *testing.T) {
	te := NewTimerEntries()

	var ranBeforeStart, ranAfterStart bool
	// Start前AddTimer:内部resetTime遇到nil Timer直接返回
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		ranBeforeStart = true
		return 0
	})
	// Start前Run:job正常执行,不panic
	te.Run()
	if !ranBeforeStart {
		t.Fatal("expired job should run before Start")
	}

	// Start后一切正常
	te.Start()
	defer te.Stop()
	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		ranAfterStart = true
		return 0
	})
	te.Run()
	if !ranAfterStart {
		t.Fatal("job should run after Start")
	}
}

// TestTimerEntries_RunUsesNowFuncBase 验证#5:Run内部使用Now()而非真实时间
// fakeNow远早于真实时间,若Run误用time.Now(),entry会被误判为早已到期而错误执行
func TestTimerEntries_RunUsesNowFuncBase(t *testing.T) {
	fakeNow := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	te := NewTimerEntriesWithArgs(func() time.Time {
		return fakeNow
	}, time.Millisecond)

	var ran bool
	te.AddTimer(fakeNow.Add(10*time.Second), func() time.Duration {
		ran = true
		return 0
	})

	te.Run()
	if ran {
		t.Fatal("job should not run: next is 10s after fake now")
	}
	if len(te.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(te.entries))
	}

	// 逻辑时间前进11秒,entry到期执行
	fakeNow = fakeNow.Add(11 * time.Second)
	te.Run()
	if !ran {
		t.Fatal("job should run after fake now advanced past next")
	}
}

// TestTimerEntries_TimeOffsetBase 验证#5:负timeOffset下Run与注册使用同一逻辑时间基准
// offset=-24h时,next=Now()+10s(即真实时间-24h+10s,早已"过期"于真实时间),
// 若Run误用真实时间判断,该entry会被立即错误执行
func TestTimerEntries_TimeOffsetBase(t *testing.T) {
	te := NewTimerEntries()
	te.SetTimeOffset(-24 * time.Hour)

	var ran bool
	te.After(10*time.Second, func() time.Duration {
		ran = true
		return 0
	})

	te.Run()
	if ran {
		t.Fatal("job should not run: next is 10s after logical now")
	}
	if len(te.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(te.entries))
	}
}

// TestTimerEntries_TimerChanRecurringWakeup 验证timer链路:Run后resetTime重新武装timer,recurring可再次唤醒
func TestTimerEntries_TimerChanRecurringWakeup(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, 5*time.Millisecond)
	te.Start()
	defer te.Stop()

	var counter int
	d := 30 * time.Millisecond
	te.After(d, func() time.Duration {
		counter++
		return d
	})

	waitFire := func() bool {
		select {
		case <-te.TimerChan():
			return true
		case <-time.After(2 * time.Second):
			return false
		}
	}

	if !waitFire() {
		t.Fatal("timer should fire first time")
	}
	te.Run()
	if counter != 1 {
		t.Fatalf("expected counter 1, got %d", counter)
	}

	// recurring重调度后,被Reset的timer应能再次触发
	if !waitFire() {
		t.Fatal("timer should fire again after recurring reschedule")
	}
	te.Run()
	if counter != 2 {
		t.Fatalf("expected counter 2, got %d", counter)
	}
}

// TestTimerEntries_ManyTimersRandomInsert 验证批量乱序注册:到期按next升序执行,未到期保留且有序
func TestTimerEntries_ManyTimersRandomInsert(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	now := time.Now()
	const total = 100
	const expired = 30
	var expiredOrder []int

	// 先添加未来的70个,再倒序添加已过期的30个
	for i := expired; i < total; i++ {
		te.AddTimer(now.Add(time.Duration(i+1)*time.Second), func() time.Duration { return 0 })
	}
	for i := expired - 1; i >= 0; i-- {
		idx := i
		te.AddTimer(now.Add(-time.Duration(idx+1)*time.Second), func() time.Duration {
			expiredOrder = append(expiredOrder, idx)
			return 0
		})
	}

	te.Run()
	if len(expiredOrder) != expired {
		t.Fatalf("expected %d expired jobs run, got %d", expired, len(expiredOrder))
	}
	// 已到期的按next升序执行:-30s(idx=29)最先,-1s(idx=0)最后
	for i, v := range expiredOrder {
		if want := expired - 1 - i; v != want {
			t.Fatalf("expired jobs should run in next-ascending order: order[%d]=%d, want %d", i, v, want)
		}
	}
	// 未到期的保留且仍有序
	if len(te.entries) != total-expired {
		t.Fatalf("expected %d remaining entries, got %d", total-expired, len(te.entries))
	}
	for i := 1; i < len(te.entries); i++ {
		if te.entries[i].next.Before(te.entries[i-1].next) {
			t.Fatalf("remaining entries not sorted at index %d", i)
		}
	}
}

// TestTimerEntries_ResetDrainsStaleValue 验证防御#1:resetTime在Reset前排空channel中未读的stale值
// 场景:timer已触发但事件循环忙于其他消息未读取,此期间注册新timer触发resetTime
func TestTimerEntries_ResetDrainsStaleValue(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, 20*time.Millisecond)
	te.Start()
	defer te.Stop()

	var firstRan, secondRan bool
	// 第一个timer很快到期,其值留在channel中未被读取(模拟事件循环忙于其他消息)
	te.After(10*time.Millisecond, func() time.Duration {
		firstRan = true
		return 0
	})
	time.Sleep(100 * time.Millisecond)

	// 消息处理期间注册新timer,内部resetTime应Stop+drain掉stale值再Reset
	te.After(30*time.Millisecond, func() time.Duration {
		secondRan = true
		return 0
	})

	// stale值已被排空:立即非阻塞读取应无值
	// (修复前这里能读到stale值,事件循环会立即Run一次空跑)
	select {
	case <-te.TimerChan():
		t.Fatal("stale value should have been drained by resetTime")
	default:
	}

	// 新timer正常触发:第一个job在第一次唤醒执行,
	// 第二个job(next在注册后30ms)可能需要再等一次唤醒,循环等待直到都执行
	deadline := time.After(2 * time.Second)
	for !(firstRan && secondRan) {
		select {
		case <-te.TimerChan():
			te.Run()
		case <-deadline:
			t.Fatalf("timers should fire, got first=%v second=%v", firstRan, secondRan)
		}
	}
}

// TestTimerEntries_TimerChanBeforeStartPanics 验证防御#3:Start前调用TimerChan快速失败
// (否则返回nil channel,select会永久静默阻塞)
func TestTimerEntries_TimerChanBeforeStartPanics(t *testing.T) {
	te := NewTimerEntries()

	defer func() {
		if err := recover(); err == nil {
			t.Fatal("TimerChan before Start should panic")
		}
	}()
	_ = te.TimerChan()
}

// TestTimerEntries_PanicHandlerKeepsRunning 验证:设置panicHandler后job panic不传播出Run,
// panic的job被移除不再重复执行,同轮后续job正常执行,组件持续可用
func TestTimerEntries_PanicHandlerKeepsRunning(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	var handlerErr any
	var handlerJob TimerJob
	te.SetPanicHandler(func(job TimerJob, err any) {
		handlerJob = job
		handlerErr = err
	})

	var ranAfterPanic bool
	var panicCount int
	te.AddTimer(time.Now().Add(-2*time.Second), func() time.Duration {
		panicCount++
		panic("job panic test")
	})
	// 同轮的后续job也应正常执行
	te.AddTimer(time.Now().Add(-1*time.Second), func() time.Duration {
		ranAfterPanic = true
		return 0
	})

	// Run不panic,正常返回
	ran := te.Run()
	if !ran {
		t.Fatal("Run should return true")
	}
	if handlerErr != "job panic test" {
		t.Fatalf("panicHandler should receive err, got %v", handlerErr)
	}
	if handlerJob == nil {
		t.Fatal("panicHandler should receive the panicking job")
	}
	if !ranAfterPanic {
		t.Fatal("job after panicking job should run in the same Run")
	}
	// panic的job与已执行的job都被移除
	if len(te.entries) != 0 {
		t.Fatalf("panicking job should be removed, got %d entries", len(te.entries))
	}

	// 再次Run:panic的job不会重复执行
	te.Run()
	if panicCount != 1 {
		t.Fatalf("panicking job should not re-execute, got %d panics", panicCount)
	}
}

// TestTimerEntries_PanicPropagatesWithoutHandler 验证默认语义:未设置panicHandler时panic正常传播
func TestTimerEntries_PanicPropagatesWithoutHandler(t *testing.T) {
	te := NewTimerEntriesWithArgs(nil, time.Millisecond)
	te.Start()
	defer te.Stop()

	te.AddTimer(time.Now().Add(-time.Second), func() time.Duration {
		panic("no handler")
	})
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("panic should propagate without panicHandler")
		}
	}()
	te.Run()
}
