package logger

import "log/slog"

func Error(message string, args ...any) { slog.Error(message, args...) }
func Warn(message string, args ...any)  { slog.Warn(message, args...) }
