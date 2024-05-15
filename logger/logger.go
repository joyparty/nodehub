package logger

// Logger 日志接口
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type nullLogger struct{}

func (nullLogger) Debug(msg string, args ...any) {}
func (nullLogger) Info(msg string, args ...any)  {}
func (nullLogger) Warn(msg string, args ...any)  {}
func (nullLogger) Error(msg string, args ...any) {}

var defaultLogger Logger = nullLogger{}

// SetLogger 设置日志实现
func SetLogger(logger Logger) {
	defaultLogger = logger
}

// Debug 打印调试日志
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info 打印信息日志
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn 打印警告日志
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error 打印错误日志
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}
