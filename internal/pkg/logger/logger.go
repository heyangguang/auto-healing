package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger

	// 分类文件写入器
	categoryWriters = make(map[Category]*lumberjack.Logger)
	categoryMu      sync.RWMutex

	// 日志目录
	logDir = "logs"
)

// Category 日志大类
type Category string

const (
	CatAPI   Category = "API"
	CatSched Category = "SCHED"
	CatExec  Category = "EXEC"
	CatSync  Category = "SYNC"
	CatAuth  Category = "AUTH"
)

// 子类常量
const (
	// API 子类
	SubAPIHeal   = "HEAL"
	SubAPIPlugin = "PLUGIN"
	SubAPIGit    = "GIT"
	SubAPIExec   = "EXEC"
	SubAPIAuth   = "AUTH"
	SubAPIUser   = "USER"
	SubAPINotify = "NOTIFY"

	// SCHED 子类
	SubSchedHeal = "HEAL"
	SubSchedSync = "SYNC"
	SubSchedGit  = "GIT"
	SubSchedTask = "TASK"

	// EXEC 子类
	SubExecFlow    = "FLOW"
	SubExecNode    = "NODE"
	SubExecAnsible = "ANSIBLE"
	SubExecTask    = "TASK"

	// SYNC 子类
	SubSyncPlugin = "PLUGIN"
	SubSyncGit    = "GIT"

	// AUTH 子类
	SubAuthSecret = "SECRET"
	SubAuthLogin  = "LOGIN"
	SubAuthToken  = "TOKEN"
)

// categoryToFile 分类对应的日志文件
var categoryToFile = map[Category]string{
	CatAPI:   "api.log",
	CatSched: "scheduler.log",
	CatExec:  "execution.log",
	CatSync:  "sync.log",
	CatAuth:  "auth.log",
}

// Init 初始化日志系统
func Init(cfg *config.LogConfig) {
	var cores []zapcore.Core

	// 解析日志级别
	level := parseLevel(cfg.Level)

	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
	}

	// 控制台输出（带颜色和 emoji）
	if cfg.Console.Enabled {
		consoleEncoder := getEncoder(cfg.Console.Format, cfg.Console.Color)
		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		)
		cores = append(cores, consoleCore)
	}

	// 文件输出（主日志文件）
	if cfg.File.Enabled {
		if err := os.MkdirAll(cfg.File.Path, 0755); err == nil {
			fileEncoder := getEncoder(cfg.File.Format, false)

			logPath := filepath.Join(cfg.File.Path, cfg.File.Filename)
			fileWriter := &lumberjack.Logger{
				Filename:   logPath,
				MaxSize:    cfg.File.MaxSize,
				MaxBackups: cfg.File.MaxBackups,
				MaxAge:     cfg.File.MaxAge,
				Compress:   cfg.File.Compress,
			}

			fileCore := zapcore.NewCore(
				fileEncoder,
				zapcore.AddSync(fileWriter),
				level,
			)
			cores = append(cores, fileCore)
		}
	}

	// 创建 logger
	if len(cores) == 0 {
		logger = zap.NewNop()
	} else {
		core := zapcore.NewTee(cores...)
		logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	}

	sugar = logger.Sugar()

	// 初始化分类文件写入器
	initCategoryWriters()
}

// initCategoryWriters 初始化各分类的文件写入器
func initCategoryWriters() {
	categoryMu.Lock()
	defer categoryMu.Unlock()

	for cat, filename := range categoryToFile {
		categoryWriters[cat] = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, filename),
			MaxSize:    100, // MB
			MaxBackups: 3,
			MaxAge:     7, // days
			Compress:   true,
		}
	}
}

// getCategoryWriter 获取分类的文件写入器
func getCategoryWriter(cat Category) *lumberjack.Logger {
	categoryMu.RLock()
	defer categoryMu.RUnlock()
	return categoryWriters[cat]
}

// getEncoder 获取编码器
func getEncoder(format string, color bool) zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "", // 不显示 caller，使日志格式统一
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

// parseLevel 解析日志级别
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

// Sync 同步日志缓冲
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// ============================================================
// 分类日志器
// ============================================================

// CategoryLogger 分类日志器
type CategoryLogger struct {
	category Category
	sub      string
}

// API 返回 API 分类日志器
func API(sub string) *CategoryLogger {
	return &CategoryLogger{category: CatAPI, sub: sub}
}

// Sched 返回调度分类日志器
func Sched(sub string) *CategoryLogger {
	return &CategoryLogger{category: CatSched, sub: sub}
}

// Exec 返回执行分类日志器
func Exec(sub string) *CategoryLogger {
	return &CategoryLogger{category: CatExec, sub: sub}
}

// Sync_ 返回同步分类日志器（Sync 已被占用）
func Sync_(sub string) *CategoryLogger {
	return &CategoryLogger{category: CatSync, sub: sub}
}

// Auth 返回认证分类日志器
func Auth(sub string) *CategoryLogger {
	return &CategoryLogger{category: CatAuth, sub: sub}
}

// formatTag 格式化标签
func (l *CategoryLogger) formatTag() string {
	if l.sub == "" {
		return fmt.Sprintf("[%s]", l.category)
	}
	return fmt.Sprintf("[%s:%s]", l.category, l.sub)
}

// writeToFile 写入分类日志文件（不带 emoji）
func (l *CategoryLogger) writeToFile(level, msg string) {
	writer := getCategoryWriter(l.category)
	if writer == nil {
		return
	}
	// 格式: 时间 级别 [TAG] 消息
	line := fmt.Sprintf("%s\t%s\t%s %s\n",
		formatTime(),
		level,
		l.formatTag(),
		msg,
	)
	_, _ = writer.Write([]byte(line))
}

// formatTime 格式化时间
func formatTime() string {
	return time.Now().Format("2006-01-02T15:04:05.000-0700")
}

// Info 输出信息日志
func (l *CategoryLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Console: 带标签
	sugar.Infof("%s %s", l.formatTag(), msg)
	// File: 写入分类文件
	l.writeToFile("INFO", msg)
}

// Debug 输出调试日志
func (l *CategoryLogger) Debug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Debugf("%s %s", l.formatTag(), msg)
	l.writeToFile("DEBUG", msg)
}

// Warn 输出警告日志
func (l *CategoryLogger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Warnf("%s %s", l.formatTag(), msg)
	l.writeToFile("WARN", msg)
}

// Error 输出错误日志
func (l *CategoryLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Errorf("%s %s", l.formatTag(), msg)
	l.writeToFile("ERROR", msg)
}

// ============================================================
// 原有兼容函数（保留）
// ============================================================

// Debug 调试日志
func Debug(format string, args ...interface{}) {
	sugar.Debugf(format, args...)
}

// Info 信息日志
func Info(format string, args ...interface{}) {
	sugar.Infof(format, args...)
}

// Warn 警告日志
func Warn(format string, args ...interface{}) {
	sugar.Warnf(format, args...)
}

// Error 错误日志
func Error(format string, args ...interface{}) {
	sugar.Errorf(format, args...)
}

// Fatal 致命错误日志
func Fatal(format string, args ...interface{}) {
	sugar.Fatalf(format, args...)
}

// Fields 用于结构化日志
type Fields map[string]interface{}

// WithFields 返回带有额外字段的日志条目
func WithFields(fields Fields) *Entry {
	return &Entry{fields: fields}
}

// Entry 日志条目
type Entry struct {
	fields Fields
}

// Info 输出带字段的信息日志
func (e *Entry) Info(msg string) {
	keysAndValues := make([]interface{}, 0, len(e.fields)*2)
	for k, v := range e.fields {
		keysAndValues = append(keysAndValues, k, v)
	}
	sugar.Infow(msg, keysAndValues...)
}

// Error 输出带字段的错误日志
func (e *Entry) Error(msg string) {
	keysAndValues := make([]interface{}, 0, len(e.fields)*2)
	for k, v := range e.fields {
		keysAndValues = append(keysAndValues, k, v)
	}
	sugar.Errorw(msg, keysAndValues...)
}

// GetZapLogger 获取底层 zap logger
func GetZapLogger() *zap.Logger {
	return logger
}
