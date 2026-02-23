package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_AllowsNormalTraffic(t *testing.T) {
	t.Parallel()

	handler := RateLimit(RateLimitConfig{RPS: 100, Burst: 10})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "ok")
		}),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimit_BlocksExcessiveTraffic(t *testing.T) {
	t.Parallel()

	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 2})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "ok")
		}),
	)

	// First 2 requests should succeed (burst=2)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// Third request should be rate limited
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	t.Parallel()

	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "ok")
		}),
	)

	// Exhaust limit for IP 1
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	handler.ServeHTTP(rec1, req1)

	rec1b := httptest.NewRecorder()
	req1b := httptest.NewRequest(http.MethodGet, "/", nil)
	req1b.RemoteAddr = "10.0.0.1:12346"
	handler.ServeHTTP(rec1b, req1b)

	// IP 2 should still be allowed
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	handler.ServeHTTP(rec2, req2)

	if rec1b.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 second request: status = %d, want %d", rec1b.Code, http.StatusTooManyRequests)
	}
	if rec2.Code != http.StatusOK {
		t.Errorf("IP2 first request: status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	t.Parallel()

	handler := RateLimit(RateLimitConfig{RPS: 1, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "ok")
		}),
	)

	// Exhaust limit
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.3:12345"
	handler.ServeHTTP(rec1, req1)

	// Should get Retry-After
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.3:12345"
	handler.ServeHTTP(rec2, req2)

	if got := rec2.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
}

func TestRateLimit_DefaultConfig(t *testing.T) {
	t.Parallel()

	handler := RateLimit()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.4:12345"
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(10, 20)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	// Should allow first request
	if !rl.Allow("1.2.3.4") {
		t.Error("first request should be allowed")
	}
}

func TestExtractIP_WithPort(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1:8080"

	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("extractIP = %q, want %q", ip, "192.168.1.1")
	}
}

func TestExtractIP_WithoutPort(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.1"

	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("extractIP = %q, want %q", ip, "192.168.1.1")
	}
}
