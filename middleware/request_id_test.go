package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/source-maker/broth/httputil"
)

func TestRequestIDGeneratesID(t *testing.T) {
	t.Parallel()

	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.Get(r.Context(), httputil.RequestID)
		if !ok {
			t.Error("request ID not found in context")
			return
		}
		capturedID = id
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	// Context should have a non-empty ID.
	if capturedID == "" {
		t.Fatal("generated request ID should not be empty")
	}

	// Response header should have the same ID.
	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("X-Request-ID response header should not be empty")
	}
	if headerID != capturedID {
		t.Errorf("response header ID = %q, context ID = %q; they should match", headerID, capturedID)
	}

	// Generated IDs should be 32 hex characters (16 bytes).
	if len(capturedID) != 32 {
		t.Errorf("generated ID length = %d, want 32", len(capturedID))
	}
}

func TestRequestIDReusesExistingHeader(t *testing.T) {
	t.Parallel()

	existingID := "my-custom-request-id-abc123"

	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.Get(r.Context(), httputil.RequestID)
		if !ok {
			t.Error("request ID not found in context")
			return
		}
		capturedID = id
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	handler.ServeHTTP(rec, req)

	// Should reuse the existing ID.
	if capturedID != existingID {
		t.Errorf("context ID = %q, want %q", capturedID, existingID)
	}

	headerID := rec.Header().Get("X-Request-ID")
	if headerID != existingID {
		t.Errorf("response header ID = %q, want %q", headerID, existingID)
	}
}

func TestRequestIDUniqueness(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool)
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 100 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rec, req)

		id := rec.Header().Get("X-Request-ID")
		if id == "" {
			t.Fatalf("request %d: X-Request-ID should not be empty", i)
		}
		if ids[id] {
			t.Fatalf("request %d: duplicate request ID %q", i, id)
		}
		ids[id] = true
	}
}

func TestRequestIDSetsResponseHeader(t *testing.T) {
	t.Parallel()

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Error("X-Request-ID response header should be set")
	}
}

func TestRequestIDEmptyHeaderGeneratesNew(t *testing.T) {
	t.Parallel()

	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := httputil.Get(r.Context(), httputil.RequestID)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Explicitly set empty header.
	req.Header.Set("X-Request-ID", "")
	handler.ServeHTTP(rec, req)

	// Empty header should result in a new generated ID.
	if capturedID == "" {
		t.Error("should generate a new ID when header is empty")
	}
	if len(capturedID) != 32 {
		t.Errorf("generated ID length = %d, want 32", len(capturedID))
	}
}

func TestRequestIDContextPropagation(t *testing.T) {
	t.Parallel()

	// Ensure the context flows to downstream handlers.
	var innerCtxID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.Get(r.Context(), httputil.RequestID)
		if !ok {
			t.Error("request ID not in context for inner handler")
			return
		}
		innerCtxID = id
	})

	handler := RequestID(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/downstream", nil)
	req.Header.Set("X-Request-ID", "propagated-id-456")
	handler.ServeHTTP(rec, req)

	if innerCtxID != "propagated-id-456" {
		t.Errorf("inner handler context ID = %q, want %q", innerCtxID, "propagated-id-456")
	}
}
