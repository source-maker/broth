package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func newTestLogger(t *testing.T, buf *bytes.Buffer, level slog.Level) *Logger {
	t.Helper()
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	return &Logger{slog: slog.New(h)}
}

func parseLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("failed to parse log line %q: %v", line, err)
	}
	return m
}

func TestNew(t *testing.T) {
	logger := New(slog.LevelInfo)
	if logger == nil {
		t.Fatal("New returned nil")
	}
	if logger.Slog() == nil {
		t.Fatal("Slog() returned nil")
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelInfo)

	logger.Info(context.Background(), "server started", "port", 8080)

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if m["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", m["level"])
	}
	if m["msg"] != "server started" {
		t.Errorf("msg = %v, want %q", m["msg"], "server started")
	}
	if port, ok := m["port"].(float64); !ok || port != 8080 {
		t.Errorf("port = %v, want 8080", m["port"])
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelError)

	logger.Error(context.Background(), "db failed", "err", "timeout")

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
	if m["msg"] != "db failed" {
		t.Errorf("msg = %v, want %q", m["msg"], "db failed")
	}
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelDebug)

	logger.Debug(context.Background(), "cache miss", "key", "user:42")

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if m["level"] != "DEBUG" {
		t.Errorf("level = %v, want DEBUG", m["level"])
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelWarn)

	logger.Warn(context.Background(), "rate limit", "remaining", 5)

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if m["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", m["level"])
	}
}

func TestLevelFiltering_DebugSuppressedAtInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelInfo)

	logger.Debug(context.Background(), "should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output for DEBUG at INFO level, got %q", buf.String())
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelInfo)

	child := logger.With("module", "account")
	child.Info(context.Background(), "user created")

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if m["module"] != "account" {
		t.Errorf("module = %v, want %q", m["module"], "account")
	}
	if m["msg"] != "user created" {
		t.Errorf("msg = %v, want %q", m["msg"], "user created")
	}
}

func TestWith_DoesNotAffectParent(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelInfo)

	_ = logger.With("child_key", "child_value")
	logger.Info(context.Background(), "parent log")

	m := parseLogLine(t, strings.TrimSpace(buf.String()))
	if _, exists := m["child_key"]; exists {
		t.Error("parent log should not contain child_key")
	}
}

func TestMultipleLogLines(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(t, &buf, slog.LevelInfo)

	logger.Info(context.Background(), "first")
	logger.Info(context.Background(), "second")
	logger.Warn(context.Background(), "third")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(lines))
	}

	m1 := parseLogLine(t, lines[0])
	m2 := parseLogLine(t, lines[1])
	m3 := parseLogLine(t, lines[2])

	if m1["msg"] != "first" {
		t.Errorf("line 1 msg = %v, want %q", m1["msg"], "first")
	}
	if m2["msg"] != "second" {
		t.Errorf("line 2 msg = %v, want %q", m2["msg"], "second")
	}
	if m3["msg"] != "third" || m3["level"] != "WARN" {
		t.Errorf("line 3 = %v, want msg=%q level=WARN", m3, "third")
	}
}
