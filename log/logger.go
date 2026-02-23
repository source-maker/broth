// Package log provides a structured logger wrapping slog.
package log

import (
	"context"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide a consistent logging interface.
type Logger struct {
	slog *slog.Logger
}

// New creates a Logger with the given minimum level.
func New(level slog.Level) *Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return &Logger{slog: slog.New(h)}
}

// Slog returns the underlying *slog.Logger for interop.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}

// Info logs at INFO level.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.slog.InfoContext(ctx, msg, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.slog.ErrorContext(ctx, msg, args...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.slog.DebugContext(ctx, msg, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.slog.WarnContext(ctx, msg, args...)
}

// With returns a new Logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{slog: l.slog.With(args...)}
}
