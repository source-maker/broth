package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_DefaultHeaders(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "0",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=()",
	}

	for header, want := range checks {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestSecurityHeaders_CSP_SSRContext(t *testing.T) {
	t.Parallel()

	// Set SSR context (default)
	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("CSP should be set for SSR context")
	}
	// Verify default CSP contains key directives
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'"} {
		if !containsSubstring(csp, want) {
			t.Errorf("CSP should contain %q, got %q", want, csp)
		}
	}
}

func TestSecurityHeaders_CSP_APIContext(t *testing.T) {
	t.Parallel()

	// Wrap with ContextDetect to set API context
	handler := ContextDetect()(SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	})))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	handler.ServeHTTP(rec, req)

	if csp := rec.Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("CSP should NOT be set for API context, got %q", csp)
	}
}

func TestSecurityHeaders_CSP_ExplicitAPIContext(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Manually set API context
	ctx := context.WithValue(req.Context(), requestContextKey{}, ContextAPI)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if csp := rec.Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("CSP should NOT be set for API context, got %q", csp)
	}
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	t.Parallel()

	customCSP := "default-src 'self'; script-src 'self' https://cdn.example.com"
	handler := SecurityHeaders(SecurityHeadersConfig{
		ContentSecurityPolicy: customCSP,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Security-Policy"); got != customCSP {
		t.Errorf("CSP = %q, want %q", got, customCSP)
	}
}

func TestSecurityHeaders_HSTS_Disabled(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS should not be set by default, got %q", got)
	}
}

func TestSecurityHeaders_HSTS_Enabled(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders(SecurityHeadersConfig{
		HSTSEnabled: true,
		HSTSMaxAge:  31536000,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	want := "max-age=31536000; includeSubDomains"
	if got := rec.Header().Get("Strict-Transport-Security"); got != want {
		t.Errorf("HSTS = %q, want %q", got, want)
	}
}

func TestSecurityHeaders_HSTS_DefaultMaxAge(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders(SecurityHeadersConfig{
		HSTSEnabled: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	want := "max-age=63072000; includeSubDomains"
	if got := rec.Header().Get("Strict-Transport-Security"); got != want {
		t.Errorf("HSTS = %q, want %q", got, want)
	}
}

func TestSecurityHeaders_HandlerCanOverride(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	// Handler's Set overwrites middleware's Set
	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want %q (handler should override)", got, "SAMEORIGIN")
	}
}

func TestSecurityHeaders_PassthroughBody(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "created")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != "created" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "created")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
