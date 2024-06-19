package logger

import "context"

type loggerKey struct{}

// NewContext 创建一个新的上下文，将logger存入其中
func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// FromContext 从上下文中获取logger
func FromContext(ctx context.Context) Logger {
	if v, ok := ctx.Value(loggerKey{}).(Logger); ok {
		return v
	}

	return defaultLogger
}

// DebugContext 打印调试日志
func DebugContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}

// InfoContext 打印信息日志
func InfoContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

// WarnContext 打印警告日志
func WarnContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

// ErrorContext 打印错误日志
func ErrorContext(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}
