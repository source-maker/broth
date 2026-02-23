package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectRequestContext_ContentTypeJSON(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/json")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("Content-Type application/json → %v, want api", got)
	}
}

func TestDetectRequestContext_ContentTypeJSONCharset(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("Content-Type with charset → %v, want api", got)
	}
}

func TestDetectRequestContext_AcceptJSON(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("Accept application/json → %v, want api", got)
	}
}

func TestDetectRequestContext_AcceptBothHTMLAndJSON(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "text/html, application/json")

	if got := DetectRequestContext(r); got != ContextSSR {
		t.Errorf("Accept with both html and json → %v, want ssr", got)
	}
}

func TestDetectRequestContext_BearerToken(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.token")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("Bearer token → %v, want api", got)
	}
}

func TestDetectRequestContext_BasicAuth(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	if got := DetectRequestContext(r); got != ContextSSR {
		t.Errorf("Basic auth → %v, want ssr", got)
	}
}

func TestDetectRequestContext_XRequestedWith(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("XMLHttpRequest → %v, want api", got)
	}
}

func TestDetectRequestContext_NoHeaders(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	if got := DetectRequestContext(r); got != ContextSSR {
		t.Errorf("no headers → %v, want ssr", got)
	}
}

func TestDetectRequestContext_HTMLAcceptOnly(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "text/html")

	if got := DetectRequestContext(r); got != ContextSSR {
		t.Errorf("Accept text/html → %v, want ssr", got)
	}
}

func TestDetectRequestContext_PriorityContentTypeOverAccept(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/html")

	if got := DetectRequestContext(r); got != ContextAPI {
		t.Errorf("Content-Type json + Accept html → %v, want api (Content-Type has priority)", got)
	}
}

func TestRequestContext_String(t *testing.T) {
	t.Parallel()

	if got := ContextSSR.String(); got != "ssr" {
		t.Errorf("ContextSSR.String() = %q, want %q", got, "ssr")
	}
	if got := ContextAPI.String(); got != "api" {
		t.Errorf("ContextAPI.String() = %q, want %q", got, "api")
	}
}

func TestGetRequestContext_DefaultSSR(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if got := GetRequestContext(ctx); got != ContextSSR {
		t.Errorf("empty context → %v, want ssr", got)
	}
}

func TestContextDetect_Middleware(t *testing.T) {
	t.Parallel()

	var captured RequestContext
	handler := ContextDetect()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetRequestContext(r.Context())
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if captured != ContextAPI {
		t.Errorf("middleware captured = %v, want api", captured)
	}
}

func TestContextDetect_MiddlewareSSR(t *testing.T) {
	t.Parallel()

	var captured RequestContext
	handler := ContextDetect()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetRequestContext(r.Context())
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("Accept", "text/html")
	handler.ServeHTTP(rec, req)

	if captured != ContextSSR {
		t.Errorf("middleware captured = %v, want ssr", captured)
	}
}

func TestForceContext_OverridesDetection(t *testing.T) {
	t.Parallel()

	var captured RequestContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetRequestContext(r.Context())
		io.WriteString(w, "ok")
	})

	// Force API context even for an SSR-looking request
	handler := ContextDetect()(ForceContext(ContextAPI, inner))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("Accept", "text/html")
	handler.ServeHTTP(rec, req)

	if captured != ContextAPI {
		t.Errorf("ForceContext(API) → %v, want api", captured)
	}
}

func TestForceContext_SSR(t *testing.T) {
	t.Parallel()

	var captured RequestContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetRequestContext(r.Context())
		io.WriteString(w, "ok")
	})

	// Force SSR context even for an API-looking request
	handler := ContextDetect()(ForceContext(ContextSSR, inner))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if captured != ContextSSR {
		t.Errorf("ForceContext(SSR) → %v, want ssr", captured)
	}
}

func TestDetectRequestContext_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    RequestContext
	}{
		{
			name:    "empty headers → SSR",
			headers: nil,
			want:    ContextSSR,
		},
		{
			name:    "form post → SSR",
			headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			want:    ContextSSR,
		},
		{
			name:    "multipart form → SSR",
			headers: map[string]string{"Content-Type": "multipart/form-data"},
			want:    ContextSSR,
		},
		{
			name:    "JSON content type → API",
			headers: map[string]string{"Content-Type": "application/json"},
			want:    ContextAPI,
		},
		{
			name:    "JSON accept only → API",
			headers: map[string]string{"Accept": "application/json"},
			want:    ContextAPI,
		},
		{
			name:    "JSON accept + HTML accept → SSR",
			headers: map[string]string{"Accept": "text/html, application/json"},
			want:    ContextSSR,
		},
		{
			name:    "Bearer token → API",
			headers: map[string]string{"Authorization": "Bearer tok123"},
			want:    ContextAPI,
		},
		{
			name:    "Basic auth → SSR",
			headers: map[string]string{"Authorization": "Basic abc"},
			want:    ContextSSR,
		},
		{
			name:    "XMLHttpRequest → API",
			headers: map[string]string{"X-Requested-With": "XMLHttpRequest"},
			want:    ContextAPI,
		},
		{
			name:    "wrong X-Requested-With → SSR",
			headers: map[string]string{"X-Requested-With": "SomeOtherThing"},
			want:    ContextSSR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			if got := DetectRequestContext(r); got != tt.want {
				t.Errorf("DetectRequestContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
