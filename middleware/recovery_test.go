package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryFromPanic(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	handler.ServeHTTP(rec, req)

	// Should return 500.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Body should contain the 500 status text.
	body := strings.TrimSpace(rec.Body.String())
	if body != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("body = %q, want %q", body, http.StatusText(http.StatusInternalServerError))
	}

	// Logger should have captured the panic.
	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("log should contain 'panic recovered', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "something went wrong") {
		t.Errorf("log should contain panic message, got: %s", logOutput)
	}
}

func TestRecoveryNoPanic(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "all good")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "all good" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "all good")
	}

	// No panic log should be written.
	if strings.Contains(buf.String(), "panic recovered") {
		t.Error("log should not contain 'panic recovered' when no panic occurs")
	}
}

func TestRecoveryPanicWithNonStringValue(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(42)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic-int", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("log should contain 'panic recovered', got: %s", logOutput)
	}
}

func TestRecoveryPanicWithError(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(io.ErrUnexpectedEOF)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panic-error", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("log should contain 'panic recovered', got: %s", logOutput)
	}
}

func TestRecoveryLogsMethodAndPath(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "POST") {
		t.Errorf("log should contain method 'POST', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/api/data") {
		t.Errorf("log should contain path '/api/data', got: %s", logOutput)
	}
}
