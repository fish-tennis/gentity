package gentity

import (
	"github.com/fish-tennis/gnet"
	"runtime"
)

var _logger gnet.Logger

// 设置日志接口
func SetLogger(log gnet.Logger) {
	_logger = log
}

func GetLogger() gnet.Logger {
	if _logger == nil {
		return gnet.GetLogger()
	}
	return _logger
}

func LogStack() {
	buf := make([]byte, 1<<12)
	GetLogger().Error(string(buf[:runtime.Stack(buf, false)]))
}