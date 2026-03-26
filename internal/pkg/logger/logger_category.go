package logger

import (
	"fmt"
	"os"
	"time"
)

func API(sub string) *CategoryLogger   { return &CategoryLogger{category: CatAPI, sub: sub} }
func Sched(sub string) *CategoryLogger { return &CategoryLogger{category: CatSched, sub: sub} }
func Exec(sub string) *CategoryLogger  { return &CategoryLogger{category: CatExec, sub: sub} }
func Sync_(sub string) *CategoryLogger { return &CategoryLogger{category: CatSync, sub: sub} }
func Auth(sub string) *CategoryLogger  { return &CategoryLogger{category: CatAuth, sub: sub} }

func (l *CategoryLogger) formatTag() string {
	if l.sub == "" {
		return fmt.Sprintf("[%s]", l.category)
	}
	return fmt.Sprintf("[%s:%s]", l.category, l.sub)
}

func (l *CategoryLogger) writeToFile(level, msg string) {
	writer := getCategoryWriter(l.category)
	if writer == nil {
		return
	}
	line := fmt.Sprintf("%s\t%s\t%s %s\n", formatTime(), level, l.formatTag(), msg)
	if _, err := writer.Write([]byte(line)); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "写入分类日志失败: %v\n", err)
	}
}

func formatTime() string {
	return time.Now().Format("2006-01-02T15:04:05.000-0700")
}

func (l *CategoryLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Infof("%s %s", l.formatTag(), msg)
	l.writeToFile("INFO", msg)
}

func (l *CategoryLogger) Debug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Debugf("%s %s", l.formatTag(), msg)
	l.writeToFile("DEBUG", msg)
}

func (l *CategoryLogger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Warnf("%s %s", l.formatTag(), msg)
	l.writeToFile("WARN", msg)
}

func (l *CategoryLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	sugar.Errorf("%s %s", l.formatTag(), msg)
	l.writeToFile("ERROR", msg)
}

func Debug(format string, args ...interface{}) { sugar.Debugf(format, args...) }
func Info(format string, args ...interface{})  { sugar.Infof(format, args...) }
func Warn(format string, args ...interface{})  { sugar.Warnf(format, args...) }
func Error(format string, args ...interface{}) { sugar.Errorf(format, args...) }
func Fatal(format string, args ...interface{}) { sugar.Fatalf(format, args...) }
