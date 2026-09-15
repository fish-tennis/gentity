package gentity

import (
	"sort"
	"time"
)

// 计时管理
// 参考https://github.com/robfig/cron
// 与cron主要用于任务计划不同,TimerEntries主要用于倒计时的管理
// 如果放在玩家的独立协程中使用,则倒计时的回调可以保证协程安全,使玩家的倒计时回调更简单
//
// 并发契约(重要):本类型完全无锁,所有方法(AddTimer/After/Start/Stop/Run等)
// 必须在实体自己的协程中调用,或者在实体协程启动前(Start之前)由调用方协程调用;
// 协程启动后从其他协程调用AddTimer/After,会与实体协程的Start/Run构成数据竞争——
// 跨协程注册定时器应改为向实体投递消息,由实体协程在消息处理中调用
//
// example:
//
//	go func() {
//	  defer timerEntries.Stop()
//	  timerEntries := NewTimerEntries()
//	  timerEntries.Start()
//	  for {
//	      select {
//	      case <-timerEntries.TimerChan():
//	           timerEntries.Run()
//	      case ...
//	      }
//	  }
//	}
type TimerEntries struct {
	entries []*timerEntry
	Timer   *time.Timer
	// 获取当前时间的接口,默认使用time.Now()
	nowFunc func() time.Time
	// resetTime时的最小间隔,默认1秒
	minInterval time.Duration
	// 时间偏差
	timeOffset time.Duration
	// Run是否正在执行
	// Run期间,addEntry只追加不sort/resetTime,避免遍历中的entries被重排导致跳过到期timer;
	// Run结束后统一sort+resetTime
	running bool
}

func NewTimerEntries() *TimerEntries {
	return &TimerEntries{
		minInterval: time.Second,
	}
}

func NewTimerEntriesWithArgs(nowFunc func() time.Time, minInterval time.Duration) *TimerEntries {
	return &TimerEntries{
		nowFunc:     nowFunc,
		minInterval: minInterval,
	}
}

// 倒计时回调函数
// 返回值:下一次执行的时间间隔,返回0表示该回调不会继续执行
type TimerJob func() time.Duration

// 时间和回调函数
// 参考https://github.com/robfig/cron
type timerEntry struct {
	next time.Time
	job  TimerJob
	// 标记该entry待删除
	// 不复用next的零值做删除标记,避免与AddTimer(time.Time{},...)的零值时间语义冲突,
	// 否则Run期间job中新加的零值时间entry会被误删
	removed bool
}

func (this *TimerEntries) GetMinInterval() time.Duration {
	return this.minInterval
}

func (this *TimerEntries) SetMinInterval(minInterval time.Duration) {
	this.minInterval = minInterval
}

func (this *TimerEntries) GetTimeOffset() time.Duration {
	return this.timeOffset
}

func (this *TimerEntries) SetTimeOffset(timeOffset time.Duration) {
	this.timeOffset = timeOffset
}

// 指定时间点执行回调
func (this *TimerEntries) AddTimer(t time.Time, f TimerJob) {
	this.addEntry(&timerEntry{next: t, job: f})
}

// 现在往后多少时间执行回调
func (this *TimerEntries) After(d time.Duration, f TimerJob) {
	this.addEntry(&timerEntry{next: this.Now().Add(d), job: f})
}

func (this *TimerEntries) addEntry(entry *timerEntry) {
	// Run期间只追加到尾部,不插入排序,避免遍历中的entries被移动导致跳过/重复执行timer
	// Run结束后会统一sort+resetTime
	if this.running {
		this.entries = append(this.entries, entry)
		return
	}
	// entries保持按next升序的不变量,二分查找插入位置,避免每次注册都全量sort
	// O(n)插入比O(n log n)的sort.Slice更快,也省去sort.Slice的reflect交换开销
	i := sort.Search(len(this.entries), func(j int) bool {
		return this.entries[j].next.After(entry.next)
	})
	this.entries = append(this.entries, nil)
	copy(this.entries[i+1:], this.entries[i:])
	this.entries[i] = entry
	this.resetTime(this.Now())
}

func (this *TimerEntries) Now() time.Time {
	if this.nowFunc == nil {
		if this.timeOffset == 0 {
			return time.Now()
		} else {
			return time.Now().Add(this.timeOffset)
		}
	}
	return this.nowFunc()
}

func (this *TimerEntries) sort() {
	sort.Slice(this.entries, func(i, j int) bool {
		return this.entries[i].next.Before(this.entries[j].next)
	})
}

func (this *TimerEntries) resetTime(now time.Time) {
	// Start之前调用时Timer可能为nil(如Start前AddTimer/Run)
	if this.Timer == nil {
		return
	}
	if len(this.entries) == 0 {
		this.Timer.Reset(time.Hour * 100000)
	} else {
		d := this.entries[0].next.Sub(now)
		if d < this.minInterval {
			d = this.minInterval
		}
		this.Timer.Reset(d)
	}
}

func (this *TimerEntries) Start() {
	this.sort()
	if len(this.entries) == 0 {
		this.Timer = time.NewTimer(time.Hour * 100000)
	} else {
		// 以最快到期的时间间隔创建一个NewTimer
		this.Timer = time.NewTimer(this.entries[0].next.Sub(this.Now()))
	}
}

func (this *TimerEntries) Stop() {
	if this.Timer != nil {
		this.Timer.Stop()
	}
}

func (this *TimerEntries) TimerChan() <-chan time.Time {
	return this.Timer.C
}

// 执行到期的timer回调
// 内部使用Now()作为当前时间,保证与AddTimer/After注册时的next时间基准一致
// (entries的next由Now()计算,可能带timeOffset或自定义nowFunc;
// 而timer通道的时间是真实墙上时钟,两者基准不同,不能混用)
func (this *TimerEntries) Run() bool {
	now := this.Now()
	removed := false
	modified := false
	jobRun := false
	// 标记Run正在执行,期间addEntry只追加不sort/resetTime
	// 用defer保证job panic时也能恢复,否则running永久为true,后续addEntry不再sort/resetTime
	this.running = true
	defer func() { this.running = false }()
	entryCount := len(this.entries)
	for i := 0; i < entryCount; i++ {
		entry := this.entries[i]
		if entry.next.After(now) {
			break
		}
		// job()里面可能执行append(entries,...)
		// 新加的entry下次Run才能被执行
		d := entry.job()
		jobRun = true
		if d > 0 {
			entry.next = now.Add(d)
		} else {
			entry.removed = true
			removed = true
		}
		modified = true
	}
	if removed {
		// 删除过期的timer
		// 倒序遍历,每次删除只移动少量元素,整体O(n)
		for i := len(this.entries) - 1; i >= 0; i-- {
			if this.entries[i].removed {
				this.entries = append(this.entries[:i], this.entries[i+1:]...)
			}
		}
	}
	// 重新排序,重置timer
	if modified {
		this.sort()
		this.resetTime(now)
	} else if len(this.entries) > entryCount {
		// Run期间有新entry被追加且没有modified(本轮没有任何job执行),
		// 也需要sort+resetTime以纳入新entry
		this.sort()
		this.resetTime(now)
	}
	return jobRun
}
