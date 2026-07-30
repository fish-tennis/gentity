package gentity

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// 独立协程的实体接口
type RoutineEntity interface {
	Entity

	// push a Message
	// 将会在RoutineEntity的独立协程中被调用
	PushMessage(message any)

	// 开启消息处理协程
	// 每个RoutineEntity一个独立的消息处理协程
	RunProcessRoutine(routineEntity RoutineEntity, routineArgs *RoutineEntityRoutineArgs) bool

	// 停止协程
	Stop()

	// 是否已停止
	// 返回true表示协程已停止或正在停止,此时PushMessage系列方法会拒绝新消息
	IsStopped() bool
}

// RoutineEntity协程参数
type RoutineEntityRoutineArgs struct {
	// 初始化,返回false时,协程不会启动
	InitFunc func(routineEntity RoutineEntity) bool
	// 消息处理函数
	ProcessMessageFunc func(routineEntity RoutineEntity, message any)
	// 有计时函数执行后调用
	AfterTimerExecuteFunc func(routineEntity RoutineEntity, t time.Time)
	// 协程结束时调用
	EndFunc func(routineEntity RoutineEntity)
}

// 独立协程的实体
type BaseRoutineEntity struct {
	BaseEntity
	// 消息队列
	messages chan any
	stopChan chan struct{}
	stopOnce sync.Once
	// 停止标志,Stop后置位,用于让PushMessage系列方法快速拒绝新消息,避免消息泄漏到无人消费的channel
	stopped atomic.Bool
	// 计时管理
	timerEntries *TimerEntries
}

func NewRoutineEntity(messageChanLen int) *BaseRoutineEntity {
	return &BaseRoutineEntity{
		messages:     make(chan any, messageChanLen),
		stopChan:     make(chan struct{}, 1),
		timerEntries: NewTimerEntries(),
	}
}

func (this *BaseRoutineEntity) GetTimerEntries() *TimerEntries {
	return this.timerEntries
}

// 停止协程
func (this *BaseRoutineEntity) Stop() {
	this.stopOnce.Do(func() {
		// 先置位stopped,让后续PushMessage系列方法快速拒绝新消息
		this.stopped.Store(true)
		this.stopChan <- struct{}{}
	})
}

// 是否已停止
func (this *BaseRoutineEntity) IsStopped() bool {
	return this.stopped.Load()
}

// push a Message
// 将会在RoutineEntity的独立协程中被调用
//
// 注意:当messages channel满时,本方法会阻塞调用方协程,直到实体协程消费消息。
// 跨协程调用时,如果实体协程卡住(死循环/慢IO/panic等),调用方将永久阻塞,并可能连锁扩散到整个进程。
// 跨协程场景建议优先使用 TryPushMessage 或 PushMessageTimeout。
func (this *BaseRoutineEntity) PushMessage(message any) {
	if this.stopped.Load() {
		GetLogger().Warn("PushMessage dropped, entity stopped: %v", this.GetId())
		return
	}
	GetLogger().Debug("PushMessage %v", message)
	this.messages <- message
}

// 尝试非阻塞地推送消息
// channel满时立即返回false,不会阻塞调用方
// 适用于跨协程调用场景,调用方需自行处理入队失败(丢弃/重试/记日志等)
func (this *BaseRoutineEntity) TryPushMessage(message any) bool {
	if this.stopped.Load() {
		GetLogger().Warn("TryPushMessage dropped, entity stopped: %v", this.GetId())
		return false
	}
	select {
	case this.messages <- message:
		GetLogger().Debug("TryPushMessage %v", message)
		return true
	default:
		GetLogger().Warn("TryPushMessage failed, channel full: %v", message)
		return false
	}
}

// 带超时地推送消息
// 在timeout内无法入队则返回false
// 适用于跨协程调用场景,避免因实体协程卡住导致调用方永久阻塞
func (this *BaseRoutineEntity) PushMessageTimeout(message any, timeout time.Duration) bool {
	if this.stopped.Load() {
		GetLogger().Warn("PushMessageTimeout dropped, entity stopped: %v", this.GetId())
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case this.messages <- message:
		GetLogger().Debug("PushMessageTimeout %v", message)
		return true
	case <-timer.C:
		GetLogger().Warn("PushMessageTimeout failed, timeout: %v", message)
		return false
	}
}

// 开启消息处理协程
// 每个RoutineEntity一个独立的消息处理协程
func (this *BaseRoutineEntity) RunProcessRoutine(routineEntity RoutineEntity, routineArgs *RoutineEntityRoutineArgs) bool {
	GetLogger().Debug("RunProcessRoutine %v", this.GetId())
	if routineArgs.InitFunc != nil {
		if !routineArgs.InitFunc(routineEntity) {
			return false
		}
	}
	GetApplication().GetWaitGroup().Add(1)
	go func(ctx context.Context) {
		// recover 必须在外层 defer 函数体最前面调用,才能捕获主循环传播过来的 panic
		// 同时 WaitGroup.Done 用独立 defer 保证一定执行,避免清理逻辑 panic 导致 WaitGroup 永久泄漏
		defer func() {
			if err := recover(); err != nil {
				GetLogger().Error("recover:%v", err)
				LogStack()
			}
			// Done 独立 defer,即使下面的清理逻辑 panic 也会执行,防止 WaitGroup 死锁
			defer GetApplication().GetWaitGroup().Done()
			// 清理逻辑用独立匿名函数 + recover 保护,防止单个回调 panic 导致整个 goroutine 崩溃
			func() {
				defer func() {
					if err := recover(); err != nil {
						GetLogger().Error("cleanup recover:%v", err)
						LogStack()
					}
				}()
				this.timerEntries.Stop()
				// 协程结束的时候,清理接口
				if routineArgs.EndFunc != nil {
					routineArgs.EndFunc(routineEntity)
				}
			}()
			// 标记协程已停止,拒绝后续消息
			this.stopped.Store(true)
			GetLogger().Debug("EndProcessRoutine %v", this.GetId())
		}()

		if this.timerEntries == nil {
			this.timerEntries = NewTimerEntries()
		}
		this.timerEntries.Start()
		for {
			select {
			case <-ctx.Done():
				GetLogger().Info("exitNotify %v", this.GetId())
				goto END
			case <-this.stopChan:
				GetLogger().Debug("stop %v", this.GetId())
				goto END
			case routineMessage := <-this.messages:
				// nil消息 表示这是需要处理的最后一条消息
				if routineMessage == nil {
					return
				}
				if routineArgs.ProcessMessageFunc != nil {
					routineArgs.ProcessMessageFunc(routineEntity, routineMessage)
				}
			case timeNow := <-this.timerEntries.TimerChan():
				// 计时器的回调在RoutineEntity协程里执行,所以是协程安全的
				if this.timerEntries.Run(timeNow) {
					if routineArgs.AfterTimerExecuteFunc != nil {
						routineArgs.AfterTimerExecuteFunc(routineEntity, timeNow)
					}
				}
			}
		}

		// 有可能还有未处理的消息
	END:
		messageLen := len(this.messages)
		for i := 0; i < messageLen; i++ {
			routineMessage := <-this.messages
			// nil消息 表示这是需要处理的最后一条消息
			if routineMessage == nil {
				return
			}
			if routineArgs.ProcessMessageFunc != nil {
				routineArgs.ProcessMessageFunc(routineEntity, routineMessage)
			}
		}
	}(GetApplication().GetContext())
	return true
}
