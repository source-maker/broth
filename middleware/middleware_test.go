package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChainOrder(t *testing.T) {
	t.Parallel()

	var order []string

	mwA := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "A-before")
			next.ServeHTTP(w, r)
			order = append(order, "A-after")
		})
	}
	mwB := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "B-before")
			next.ServeHTTP(w, r)
			order = append(order, "B-after")
		})
	}
	mwC := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "C-before")
			next.ServeHTTP(w, r)
			order = append(order, "C-after")
		})
	}

	chained := Chain(mwA, mwB, mwC)
	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	// A is outermost, C is innermost.
	want := []string{"A-before", "B-before", "C-before", "handler", "C-after", "B-after", "A-after"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d; got %v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestChainEmpty(t *testing.T) {
	t.Parallel()

	chained := Chain()
	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "passthrough")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != "passthrough" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "passthrough")
	}
}

func TestChainSingle(t *testing.T) {
	t.Parallel()

	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	chained := Chain(mw)
	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("single middleware was not called")
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestChainHeaderPropagation(t *testing.T) {
	t.Parallel()

	addHeader := func(key, value string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(key, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	chained := Chain(
		addHeader("X-First", "1"),
		addHeader("X-Second", "2"),
	)

	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-First"); got != "1" {
		t.Errorf("X-First = %q, want %q", got, "1")
	}
	if got := rec.Header().Get("X-Second"); got != "2" {
		t.Errorf("X-Second = %q, want %q", got, "2")
	}
}

func TestChainShortCircuit(t *testing.T) {
	t.Parallel()

	handlerCalled := false

	blocker := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do NOT call next -- short-circuit.
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "blocked")
		})
	}

	chained := Chain(blocker)
	handler := chained(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("handler was called despite short-circuit middleware")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Body.String() != "blocked" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "blocked")
	}
}
