package gentity

import (
	"context"
	"sync"
	"time"
)

// 独立协程的实体接口
type RoutineEntity interface {
	Entity

	// push a Message
	// 将会在RoutineEntity的独立协程中被调用
	PushMessage(message any)

	//// push a Message and return the reply chan
	//PushMessageBlock(message any) <-chan gnet.Packet

	// 开启消息处理协程
	// 每个RoutineEntity一个独立的消息处理协程
	RunProcessRoutine(routineEntity RoutineEntity, routineArgs *RoutineEntityRoutineArgs) bool

	// 停止协程
	Stop()
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

//type RoutineMessage struct {
//	Message any
//	Reply   chan gnet.Packet // use for block mode
//}
//
//func (rm *RoutineMessage) PutReply(reply gnet.Packet) {
//	if rm.Reply == nil {
//		return
//	}
//	rm.Reply <- reply
//}

// 独立协程的实体
type BaseRoutineEntity struct {
	BaseEntity
	// 消息队列
	messages chan any
	stopChan chan struct{}
	stopOnce sync.Once
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
		this.stopChan <- struct{}{}
	})
}

// push a Message
// 将会在RoutineEntity的独立协程中被调用
func (this *BaseRoutineEntity) PushMessage(message any) {
	GetLogger().Debug("PushMessage %v", message)
	this.messages <- message
}

//func (this *BaseRoutineEntity) PushMessageBlock(message any) <-chan gnet.Packet {
//	GetLogger().Debug("PushMessageBlock %v", message)
//	blockMessage := &RoutineMessage{
//		Message: message,
//		Reply:   make(chan gnet.Packet, 1),
//	}
//	this.messages <- blockMessage
//	return blockMessage.Reply
//}

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
		defer func() {
			this.timerEntries.Stop()
			// 协程结束的时候,清理接口
			if routineArgs.EndFunc != nil {
				routineArgs.EndFunc(routineEntity)
			}
			GetApplication().GetWaitGroup().Done()
			if err := recover(); err != nil {
				GetLogger().Error("recover:%v", err)
				LogStack()
			}
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
