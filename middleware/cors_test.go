package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func apiContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), requestContextKey{}, ContextAPI)
	return r.WithContext(ctx)
}

func TestCORS_AllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	req.Header.Set("Origin", "https://example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("ACAO = %q, want %q", got, "https://example.com")
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	req.Header.Set("Origin", "https://evil.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO should be empty for disallowed origin, got %q", got)
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"*"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	req.Header.Set("Origin", "https://any.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://any.com" {
		t.Errorf("ACAO = %q, want %q", got, "https://any.com")
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		MaxAge:         3600,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodOptions, "/api/data", nil))
	req.Header.Set("Origin", "https://example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Allow-Methods should be set for preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Allow-Headers should be set for preflight")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("Max-Age = %q, want %q", got, "3600")
	}
}

func TestCORS_SkipsSSRContext(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"*"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	// Default context is SSR
	req.Header.Set("Origin", "https://example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO should not be set for SSR, got %q", got)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"*"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	// No Origin header
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO should not be set without Origin, got %q", got)
	}
}

func TestCORS_AllowCredentials(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	req.Header.Set("Origin", "https://example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want %q", got, "true")
	}
}

func TestCORS_VaryHeader(t *testing.T) {
	t.Parallel()

	handler := CORS(CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := apiContext(httptest.NewRequest(http.MethodGet, "/api/data", nil))
	req.Header.Set("Origin", "https://example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q", got, "Origin")
	}
}
