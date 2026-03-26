package logger

import "go.uber.org/zap"

type Fields map[string]interface{}

func WithFields(fields Fields) *Entry {
	return &Entry{fields: fields}
}

type Entry struct {
	fields Fields
}

func (e *Entry) Info(msg string) {
	logger.With(e.zapFields()...).Info(msg)
}

func (e *Entry) Error(msg string) {
	logger.With(e.zapFields()...).Error(msg)
}

func (e *Entry) zapFields() []zap.Field {
	fields := make([]zap.Field, 0, len(e.fields))
	for key, value := range e.fields {
		fields = append(fields, zap.Any(key, value))
	}
	return fields
}

func GetZapLogger() *zap.Logger {
	return logger
}
