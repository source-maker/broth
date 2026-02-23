package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/source-maker/broth/httputil"
)

func TestStatusWriterCapturesStatusCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		writeCode  int
		wantStatus int
	}{
		{"200 OK", http.StatusOK, http.StatusOK},
		{"201 Created", http.StatusCreated, http.StatusCreated},
		{"204 No Content", http.StatusNoContent, http.StatusNoContent},
		{"301 Moved Permanently", http.StatusMovedPermanently, http.StatusMovedPermanently},
		{"400 Bad Request", http.StatusBadRequest, http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound, http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}
			sw.WriteHeader(tt.writeCode)

			if sw.status != tt.wantStatus {
				t.Errorf("statusWriter.status = %d, want %d", sw.status, tt.wantStatus)
			}
		})
	}
}

func TestStatusWriterDefaultStatus(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	// Without calling WriteHeader, the default should remain 200.
	if sw.status != http.StatusOK {
		t.Errorf("default status = %d, want %d", sw.status, http.StatusOK)
	}
}

func TestStatusWriterIgnoresSecondWriteHeader(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	sw.WriteHeader(http.StatusInternalServerError) // Should be ignored.

	if sw.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d (first call should win)", sw.status, http.StatusNotFound)
	}
}

func TestLoggerMiddlewareLogs(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "hello")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	// Verify the log contains expected fields.
	if !strings.Contains(logOutput, "request") {
		t.Errorf("log should contain 'request' message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "GET") {
		t.Errorf("log should contain method 'GET', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/test-path") {
		t.Errorf("log should contain path '/test-path', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "200") {
		t.Errorf("log should contain status '200', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "duration") {
		t.Errorf("log should contain 'duration', got: %s", logOutput)
	}
}

func TestLoggerMiddlewareWithRequestID(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	inner := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/with-id", nil)

	// Inject a request ID into the context, simulating RequestID middleware.
	ctx := httputil.Set(req.Context(), httputil.RequestID, "test-req-id-123")
	req = req.WithContext(ctx)

	inner.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "test-req-id-123") {
		t.Errorf("log should contain request_id 'test-req-id-123', got: %s", logOutput)
	}
}

func TestLoggerMiddlewareCapturesNon200Status(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "not found")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "404") {
		t.Errorf("log should contain status '404', got: %s", logOutput)
	}
}

func TestLoggerMiddlewareDefaultStatusWhenNoWriteHeader(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write body without calling WriteHeader -- implicit 200.
		io.WriteString(w, "implicit 200")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "200") {
		t.Errorf("log should contain status '200' for implicit OK, got: %s", logOutput)
	}
}
