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
	if cfg == nil {
		logger = zap.NewNop()
		sugar = logger.Sugar()
		logDir = defaultLogDir
		resetCategoryWriters(false)
		return
	}

	logDir = resolveLogDir(cfg)
	fileEnabled := cfg.File.Enabled
	if fileEnabled {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("创建日志目录失败: %v\n", err)
			fileEnabled = false
		}
	}
	cores := buildCores(cfg, fileEnabled)
	logger = buildLogger(cores)
	sugar = logger.Sugar()
	resetCategoryWriters(fileEnabled)
}

func buildCores(cfg *config.LogConfig, fileEnabled bool) []zapcore.Core {
	cores := make([]zapcore.Core, 0, 2)
	level := parseLevel(cfg.Level)
	if cfg.Console.Enabled {
		cores = append(cores, zapcore.NewCore(getEncoder(cfg.Console.Format, cfg.Console.Color), zapcore.AddSync(os.Stdout), level))
	}
	if fileEnabled {
		fileWriter := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, cfg.File.Filename),
			MaxSize:    cfg.File.MaxSize,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,
			Compress:   cfg.File.Compress,
		}
		cores = append(cores, zapcore.NewCore(getEncoder(cfg.File.Format, false), zapcore.AddSync(fileWriter), level))
	}
	return cores
}

func buildLogger(cores []zapcore.Core) *zap.Logger {
	if len(cores) == 0 {
		return zap.NewNop()
	}
	return zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1))
}

func resolveLogDir(cfg *config.LogConfig) string {
	if cfg.File.Path != "" {
		return cfg.File.Path
	}
	return defaultLogDir
}

func resetCategoryWriters(enabled bool) {
	categoryMu.Lock()
	defer categoryMu.Unlock()
	closeCategoryWritersLocked()
	categoryWriters = make(map[Category]*lumberjack.Logger, len(categoryToFile))
	if !enabled {
		return
	}
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

func closeCategoryWriters() {
	categoryMu.Lock()
	defer categoryMu.Unlock()
	closeCategoryWritersLocked()
}

func closeCategoryWritersLocked() {
	for cat, writer := range categoryWriters {
		if writer != nil {
			_ = writer.Close()
		}
		delete(categoryWriters, cat)
	}
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
