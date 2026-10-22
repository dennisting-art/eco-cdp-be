package logger

import (
	"context"
	"log/slog"
)

type traceKey struct{}

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey{}, id)
}
func TraceID(ctx context.Context) string { id, _ := ctx.Value(traceKey{}).(string); return id }
func Info(ctx context.Context, message string, args ...any) {
	slog.InfoContext(ctx, message, append(args, "trace_id", TraceID(ctx))...)
}
func Warn(ctx context.Context, message string, args ...any) {
	slog.WarnContext(ctx, message, append(args, "trace_id", TraceID(ctx))...)
}
func Error(ctx context.Context, message string, args ...any) {
	slog.ErrorContext(ctx, message, append(args, "trace_id", TraceID(ctx))...)
}
