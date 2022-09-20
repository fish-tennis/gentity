package gentity

import "runtime"

var logger Logger

type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// 设置日志接口
func SetLogger(log Logger) {
	logger = log
}

func GetLogger() Logger {
	return logger
}

func Debug(format string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Error(format, args...)
}

func LogStack() {
	if logger == nil {
		return
	}
	buf := make([]byte, 1<<12)
	logger.Error(string(buf[:runtime.Stack(buf, false)]))
}