package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/source-maker/broth/session"
)

func withSession(r *http.Request) *http.Request {
	sess := &session.Session{
		ID:   "test-session",
		Data: make(map[string]any),
	}
	ctx := session.NewContext(r.Context(), sess)
	return r.WithContext(ctx)
}

func TestCSRF_GETSetsToken(t *testing.T) {
	t.Parallel()

	var gotToken string
	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = GetCSRFToken(r.Context())
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := withSession(httptest.NewRequest(http.MethodGet, "/form", nil))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotToken == "" {
		t.Error("CSRF token should be set in context for GET requests")
	}
}

func TestCSRF_POSTValidToken(t *testing.T) {
	t.Parallel()

	var csrfToken string
	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrfToken = GetCSRFToken(r.Context())
		io.WriteString(w, "ok")
	}))

	// First GET to obtain token
	rec1 := httptest.NewRecorder()
	req1 := withSession(httptest.NewRequest(http.MethodGet, "/form", nil))
	handler.ServeHTTP(rec1, req1)
	sess := session.FromContext(req1.Context())

	// POST with valid token
	form := url.Values{"_csrf_token": {csrfToken}}
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx2 := session.NewContext(req2.Context(), sess)
	req2 = req2.WithContext(ctx2)
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestCSRF_POSTInvalidToken(t *testing.T) {
	t.Parallel()

	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	// GET to set token
	rec1 := httptest.NewRecorder()
	req1 := withSession(httptest.NewRequest(http.MethodGet, "/form", nil))
	handler.ServeHTTP(rec1, req1)
	sess := session.FromContext(req1.Context())

	// POST with wrong token
	form := url.Values{"_csrf_token": {"wrong-token"}}
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx2 := session.NewContext(req2.Context(), sess)
	req2 = req2.WithContext(ctx2)
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec2.Code, http.StatusForbidden)
	}
}

func TestCSRF_POSTMissingToken(t *testing.T) {
	t.Parallel()

	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	// GET to set token
	rec1 := httptest.NewRecorder()
	req1 := withSession(httptest.NewRequest(http.MethodGet, "/form", nil))
	handler.ServeHTTP(rec1, req1)
	sess := session.FromContext(req1.Context())

	// POST without token
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/form", nil)
	ctx2 := session.NewContext(req2.Context(), sess)
	req2 = req2.WithContext(ctx2)
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec2.Code, http.StatusForbidden)
	}
}

func TestCSRF_SkipsAPIContext(t *testing.T) {
	t.Parallel()

	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "api ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	// Set API context
	ctx := context.WithValue(req.Context(), requestContextKey{}, ContextAPI)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (API should skip CSRF)", rec.Code, http.StatusOK)
	}
}

func TestCSRF_HeaderToken(t *testing.T) {
	t.Parallel()

	var csrfToken string
	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrfToken = GetCSRFToken(r.Context())
		io.WriteString(w, "ok")
	}))

	// GET to obtain token
	rec1 := httptest.NewRecorder()
	req1 := withSession(httptest.NewRequest(http.MethodGet, "/form", nil))
	handler.ServeHTTP(rec1, req1)
	sess := session.FromContext(req1.Context())

	// POST with token in header
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/form", nil)
	req2.Header.Set("X-CSRF-Token", csrfToken)
	ctx2 := session.NewContext(req2.Context(), sess)
	req2 = req2.WithContext(ctx2)
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestCSRF_Disabled(t *testing.T) {
	t.Parallel()

	handler := CSRF(CSRFConfig{Enabled: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/form", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (CSRF disabled)", rec.Code, http.StatusOK)
	}
}

func TestCSRF_SafeMethods(t *testing.T) {
	t.Parallel()

	handler := CSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec := httptest.NewRecorder()
		req := withSession(httptest.NewRequest(method, "/page", nil))
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", method, rec.Code, http.StatusOK)
		}
	}
}

func TestGetCSRFToken_EmptyContext(t *testing.T) {
	t.Parallel()

	token := GetCSRFToken(context.Background())
	if token != "" {
		t.Errorf("expected empty string, got %q", token)
	}
}
