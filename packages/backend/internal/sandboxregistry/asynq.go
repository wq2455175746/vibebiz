package sandboxregistry

import (
	"fmt"
	"log/slog"
)

type AsyncLogger struct {
	Logger *slog.Logger
}

func NewAsyncLogger(logger *slog.Logger) AsyncLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return AsyncLogger{Logger: logger}
}

func (l AsyncLogger) Debug(args ...interface{}) { l.Logger.Debug(fmt.Sprint(args...)) }
func (l AsyncLogger) Info(args ...interface{})  { l.Logger.Info(fmt.Sprint(args...)) }
func (l AsyncLogger) Warn(args ...interface{})  { l.Logger.Warn(fmt.Sprint(args...)) }
func (l AsyncLogger) Error(args ...interface{}) { l.Logger.Error(fmt.Sprint(args...)) }
func (l AsyncLogger) Fatal(args ...interface{}) { l.Logger.Error(fmt.Sprint(args...)) }
