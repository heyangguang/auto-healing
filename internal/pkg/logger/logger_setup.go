package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/company/auto-healing/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func Init(cfg *config.LogConfig) {
	var cores []zapcore.Core
	level := parseLevel(cfg.Level)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
	}
	if cfg.Console.Enabled {
		cores = append(cores, zapcore.NewCore(getEncoder(cfg.Console.Format, cfg.Console.Color), zapcore.AddSync(os.Stdout), level))
	}
	if cfg.File.Enabled {
		if err := os.MkdirAll(cfg.File.Path, 0755); err == nil {
			fileWriter := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.File.Path, cfg.File.Filename),
				MaxSize:    cfg.File.MaxSize,
				MaxBackups: cfg.File.MaxBackups,
				MaxAge:     cfg.File.MaxAge,
				Compress:   cfg.File.Compress,
			}
			cores = append(cores, zapcore.NewCore(getEncoder(cfg.File.Format, false), zapcore.AddSync(fileWriter), level))
		}
	}
	if len(cores) == 0 {
		logger = zap.NewNop()
	} else {
		logger = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1))
	}
	sugar = logger.Sugar()
	initCategoryWriters()
}

func initCategoryWriters() {
	categoryMu.Lock()
	defer categoryMu.Unlock()
	for cat, filename := range categoryToFile {
		categoryWriters[cat] = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, filename),
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		}
	}
}

func getCategoryWriter(cat Category) *lumberjack.Logger {
	categoryMu.RLock()
	defer categoryMu.RUnlock()
	return categoryWriters[cat]
}

func getEncoder(format string, color bool) zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if color {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	if format == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}
