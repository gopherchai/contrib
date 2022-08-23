package logger

import "context"

type Logger interface {
	InfoX(ctx context.Context, msg string, args ...interface{})
	WarnX(ctx context.Context, msg string, args ...interface{})
	ErrorX(ctx context.Context, msg string, args ...interface{})
	FatalX(ctx context.Context, msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Error(msg string)
}
