package log

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gopherchai/contrib/lib/metadata"

	"github.com/Shopify/sarama"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pkgerr "github.com/pkg/errors"
)

type Logger struct {
	l *zap.Logger
}

var (
	l     *Logger
	lInit sync.Once
)

type LogConfig struct {
	File, InternalFile string
	Stdout             bool
	Level              int
	JsonFormat         bool
	CallerSkip         int
}

func GetDefaultLogger() *Logger {
	return l
}

type KafkaWriteSyncer struct {
	sp    sarama.SyncProducer
	topic string
}

type KafkaAsyncWriteSyncer struct {
	ap    sarama.AsyncProducer
	topic string
}

func NewKafkaAsyncWriteSyncer(ap sarama.AsyncProducer, topic string) *KafkaAsyncWriteSyncer {
	go func() {
		for err := range ap.Errors() {
			log.Printf("AsyncProducer get error:%+v\n", err)
		}
	}()
	return &KafkaAsyncWriteSyncer{
		ap:    ap,
		topic: topic,
	}

}

func (kws *KafkaAsyncWriteSyncer) Write(data []byte) (int, error) {
	m := sarama.ProducerMessage{
		Topic: kws.topic,
		Value: sarama.ByteEncoder(data),
	}

	kws.ap.Input() <- &m

	return len(data), nil
}

func (kws *KafkaAsyncWriteSyncer) Sync() error {

	err := kws.ap.Close()
	return pkgerr.Wrapf(err, "producer close meet error")
}

func NewKafkaWriteSyncer(sp sarama.SyncProducer, topic string) *KafkaWriteSyncer {
	return &KafkaWriteSyncer{
		sp:    sp,
		topic: topic,
	}
}
func (kws *KafkaWriteSyncer) Write(data []byte) (int, error) {
	m := sarama.ProducerMessage{
		Topic: kws.topic,
		Value: sarama.ByteEncoder(data),
	}

	_, _, err := kws.sp.SendMessage(&m)

	return len(data), pkgerr.Wrapf(err, "send msg:%+v meet error", string(data))
}

func (kws *KafkaWriteSyncer) Sync() error {
	return pkgerr.Wrapf(kws.sp.Close(), "producer close meet error")
}

func NewLoggerWithWriteSyncer(ws zapcore.WriteSyncer) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "timestamp",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
	level := zapcore.DebugLevel

	core := zapcore.NewCore(enc, zapcore.NewMultiWriteSyncer(ws, zapcore.AddSync(os.Stdout), zapcore.AddSync(os.Stderr)), level)

	return zap.New(core, zap.AddCaller(), zap.ErrorOutput(os.Stdout), zap.AddCallerSkip(0))
}

func NewLogger(c LogConfig) (*zap.Logger, error) {
	logLevel := zap.DebugLevel
	fileName := c.File
	internalFatalileName := c.InternalFile
	jsonFormat := c.JsonFormat
	stdout := c.Stdout
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Encoding:         "json", //"console", //
		OutputPaths:      []string{fileName},
		ErrorOutputPaths: []string{internalFatalileName},
	}
	if !jsonFormat {
		cfg.Encoding = "console"
	}

	cfg.EncoderConfig = zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "@timestamp",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if stdout {
		var enc zapcore.Encoder
		enc = zapcore.NewConsoleEncoder(cfg.EncoderConfig)
		if jsonFormat {
			enc = zapcore.NewJSONEncoder(cfg.EncoderConfig)
		}

		core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout),
			logLevel)
		l := zap.New(core, zap.AddCaller(), zap.ErrorOutput(zapcore.AddSync(os.Stdout)), zap.AddCallerSkip(c.CallerSkip))
		return l, nil
	}

	// //zap.AddCallerSkip(1),zap.AddStacktrace(zapcore.DebugLevel)
	return cfg.Build(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(zapcore.NewJSONEncoder(cfg.EncoderConfig), getWarnriter(fileName), zapcore.DebugLevel)
	}), zap.AddCallerSkip(c.CallerSkip))

}

// 日志切割
func getWarnriter(fileName string) zapcore.WriteSyncer {
	Fatal, err := rotatelogs.New(
		strings.Replace(fileName, ".log", "", -1)+"_%Y%m%d.log",
		rotatelogs.WithLinkName(fileName),
		rotatelogs.WithMaxAge(time.Hour*24*3),     //日志在机器上保留3天
		rotatelogs.WithRotationTime(time.Hour*24), //每24小时切割
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	return zapcore.AddSync(Fatal)
}

func Init(c LogConfig) {
	lInit.Do(func() {
		zl, err := NewLogger(c)
		if err != nil {
			panic(err)
		}
		l = &Logger{
			l: zl,
		}
	})
}

func CtxFields(ctx context.Context) []zap.Field {
	md := ctx.Value(metadata.CtxKey)
	//修改为以指定顺序，且值为指定类型
	m, ok := md.(map[string]interface{})
	if !ok {
		return nil
	}
	fields := make([]zap.Field, 0, 0)
	for k, v := range m {
		fields = append(fields, zap.Any(k, v))
	}
	return fields
}

func SugerLog() *zap.SugaredLogger {
	return l.l.Sugar()
}

func InfoX(ctx context.Context, msg string, fields ...zap.Field) {
	//解析ctx 输出

	l.InfoX(ctx, msg, fields...)
}
func InfoXf(ctx context.Context, formatMsg string, args ...interface{}) {

	l.InfoXf(ctx, formatMsg, args...)
}

func WarnX(ctx context.Context, msg string, fields ...zap.Field) {

	l.WarnX(ctx, msg, fields...)
}

func WarnXf(ctx context.Context, formatMsg string, args ...interface{}) {
	msg := fmt.Sprintf(formatMsg, args...)
	l.WarnX(ctx, msg)
}

func ErrorX(ctx context.Context, msg string, fields ...zap.Field) {
	l.ErrorX(ctx, msg, fields...)
}

func ErrorXf(ctx context.Context, formatMsg string, args ...interface{}) {
	l.ErrorXf(ctx, formatMsg, args...)
}

func FatalX(ctx context.Context, msg string, fields ...zap.Field) {
	l.l.Fatal(msg, fields...)
}

func FatalXf(ctx context.Context, msg string, args ...interface{}) {
	l.FatalXf(ctx, msg, args...)
}

func Sugared() *zap.SugaredLogger {
	return l.Sugared()
}
func Close() {
	l.l.Sync()
}
