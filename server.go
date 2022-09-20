package gentity

import (
	"context"
	"sync"
)

var (
	// singleton
	_server Server
)

// 服务器接口
type Server interface {

	// 服务器进程的唯一id
	GetServerId() int32

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

// 服务器回调接口
type ServerHook interface {
	OnServerInit(initArg interface{})
	OnServerExit()
}

func SetServer(server Server) {
	_server = server
}

func GetServer() Server {
	return _server
}
