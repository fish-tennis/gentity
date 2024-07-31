package gentity

import (
	"context"
	"sync"
)

var (
	// singleton
	_application Application
)

// 进程接口
type Application interface {
	// 进程的唯一id
	GetId() int32

	GetContext() context.Context

	GetWaitGroup() *sync.WaitGroup

	// 初始化
	Init(ctx context.Context, configFile string) bool

	// 运行
	Run(ctx context.Context)

	// 定时更新
	OnUpdate(ctx context.Context, updateCount int64)

	// 退出
	Exit()
}

// Application回调接口
type ApplicationHook interface {
	OnRegisterServerHandler(arg any)
	OnApplicationInit(initArg any)
	OnApplicationExit()
}

func SetApplication(application Application) {
	_application = application
}

func GetApplication() Application {
	return _application
}
