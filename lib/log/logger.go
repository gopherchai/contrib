package log

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

func (l *Logger) InfoX(ctx context.Context, msg string, fields ...zap.Field) {

	fields = append(fields, CtxFields(ctx)...)
	l.l.Info(msg, fields...)

}
func (l *Logger) Infof(msg string, args ...interface{}) {

	l.InfoXf(nil, msg, args...)

}

func (l *Logger) InfoXf(ctx context.Context, msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	l.InfoX(ctx, msg)
}

func (l *Logger) WarnXf(ctx context.Context, msg string, args ...interface{}) {
	fields := CtxFields(ctx)
	msg = fmt.Sprintf(msg, args...)

	l.l.Warn(msg, fields...)
}
func (l *Logger) WarnX(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(fields, CtxFields(ctx)...)

	l.l.Warn(msg, fields...)
}

func (l *Logger) ErrorXf(ctx context.Context, msg string, args ...interface{}) {
	fields := CtxFields(ctx)
	msg = fmt.Sprintf(msg, args...)
	l.l.Error(msg, fields...)
}

func (l *Logger) Error(msg string) {

	l.l.Error(msg)
}

func (l *Logger) ErrorX(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(fields, CtxFields(ctx)...)
	l.l.Error(msg, fields...)
}

func (l *Logger) FatalX(ctx context.Context, msg string, fields ...zap.Field) {
	fields = append(fields, CtxFields(ctx)...)
	l.l.Fatal(msg, fields...)
}

func (l *Logger) FatalXf(ctx context.Context, msg string, args ...interface{}) {
	fields := CtxFields(ctx)
	msg = fmt.Sprintf(msg, args...)
	l.l.Fatal(msg, fields...)
}

func (l *Logger) Close() {
	l.l.Sync()
}
func (l *Logger) Sugared() *zap.SugaredLogger {
	return l.l.Sugar()
}
