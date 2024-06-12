package loghelper

import (
	"fmt"
	"log/slog"
)

type LogHelper struct {
	unerlyingLogger *slog.Logger
}

func (h *LogHelper) Debug(msg string, args ...any) {
	h.unerlyingLogger.Debug(msg, args...)
}

func (h *LogHelper) Debugf(format string, args ...any) {
	h.unerlyingLogger.Debug(fmt.Sprintf(format, args...))
}

func (h *LogHelper) Info(msg string, args ...any) {
	h.unerlyingLogger.Info(msg, args...)
}

func (h *LogHelper) Infof(format string, args ...any) {
	h.unerlyingLogger.Info(fmt.Sprintf(format, args...))
}

func (h *LogHelper) Warn(msg string, args ...any) {
	h.unerlyingLogger.Warn(msg, args...)
}

func (h *LogHelper) Warnf(format string, args ...any) {
	h.unerlyingLogger.Warn(fmt.Sprintf(format, args...))
}

func (h *LogHelper) Error(msg string, args ...any) {
	h.unerlyingLogger.Error(msg, args...)
}

func (h *LogHelper) Errorf(format string, args ...any) {
	h.unerlyingLogger.Error(fmt.Sprintf(format, args...))
}

func NewLogHelper(logger *slog.Logger) *LogHelper {
	return &LogHelper{
		unerlyingLogger: logger,
	}
}
