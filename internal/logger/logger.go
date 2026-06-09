package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Loggers struct {
	App      *zap.Logger
	SyncLog  *zap.Logger
	Telegram *zap.Logger
	Error    *zap.Logger
}

func New(logDir string) (Loggers, error) {
	return newWithConsole(logDir, zapcore.Lock(os.Stdout))
}

func newWithConsole(logDir string, console zapcore.WriteSyncer) (Loggers, error) {
	errorCore := fileCore(filepath.Join(logDir, "error.log"), zapcore.ErrorLevel)
	appCores := []zapcore.Core{
		fileCore(filepath.Join(logDir, "app.log"), zapcore.DebugLevel),
		errorCore,
	}
	if console != nil {
		appCores = append(appCores, consoleCore(console, zapcore.DebugLevel))
	}
	appCore := zapcore.NewTee(
		appCores...,
	)

	logs := Loggers{
		App:      zap.New(appCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)),
		SyncLog:  zap.New(fileCore(filepath.Join(logDir, "sync.log"), zapcore.DebugLevel), zap.AddCaller()),
		Telegram: zap.New(fileCore(filepath.Join(logDir, "telegram.log"), zapcore.DebugLevel), zap.AddCaller()),
		Error:    zap.New(errorCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)),
	}
	return logs, nil
}

func Nop() Loggers {
	nop := zap.NewNop()
	return Loggers{
		App:      nop,
		SyncLog:  nop,
		Telegram: nop,
		Error:    nop,
	}
}

func (l Loggers) Sync() error {
	for _, logger := range []*zap.Logger{l.App, l.SyncLog, l.Telegram, l.Error} {
		if logger != nil {
			_ = logger.Sync()
		}
	}
	return nil
}

func fileCore(path string, level zapcore.LevelEnabler) zapcore.Core {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	writer := zapcore.AddSync(newRotatingWriter(path))
	return zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), writer, level)
}

func consoleCore(writer zapcore.WriteSyncer, level zapcore.LevelEnabler) zapcore.Core {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), writer, level)
}

func newRotatingWriter(path string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10,
		MaxBackups: 5,
		Compress:   true,
	}
}
