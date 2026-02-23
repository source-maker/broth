package middleware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/source-maker/broth/auth"
)

func testTokenService(t *testing.T) *auth.TokenService {
	t.Helper()
	return auth.NewTokenService([]byte("test-secret-key-32-bytes-long!!!"), 15*time.Minute)
}

func TestAuth_BearerToken_API(t *testing.T) {
	t.Parallel()

	ts := testTokenService(t)
	user := &auth.User{ID: 42, Email: "alice@example.com", Roles: []string{"admin"}}
	token, err := ts.GenerateAccessToken(user)
	if err != nil {
		t.Fatal(err)
	}

	var gotUser *auth.User
	handler := ContextDetect()(Auth(AuthConfig{TokenService: ts})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser = auth.GetUser(r.Context())
			io.WriteString(w, "ok")
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if gotUser == nil {
		t.Fatal("user should be in context")
	}
	if gotUser.ID != 42 {
		t.Errorf("user ID = %d, want 42", gotUser.ID)
	}
	if gotUser.Email != "alice@example.com" {
		t.Errorf("user email = %q, want %q", gotUser.Email, "alice@example.com")
	}
}

func TestAuth_BearerToken_Invalid(t *testing.T) {
	t.Parallel()

	ts := testTokenService(t)

	handler := ContextDetect()(Auth(AuthConfig{TokenService: ts})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "should not reach")
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_NoAuthHeader_Passes(t *testing.T) {
	t.Parallel()

	ts := testTokenService(t)
	var gotUser *auth.User

	handler := ContextDetect()(Auth(AuthConfig{TokenService: ts})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser = auth.GetUser(r.Context())
			io.WriteString(w, "ok")
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUser != nil {
		t.Error("user should be nil when no auth header")
	}
}

func TestAuth_SSR_NoSession(t *testing.T) {
	t.Parallel()

	var gotUser *auth.User

	handler := ContextDetect()(Auth(AuthConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser = auth.GetUser(r.Context())
			io.WriteString(w, "ok")
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("Accept", "text/html")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUser != nil {
		t.Error("user should be nil without session")
	}
}

func TestAuth_InvalidAuthorizationFormat(t *testing.T) {
	t.Parallel()

	handler := ContextDetect()(Auth(AuthConfig{TokenService: testTokenService(t)})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "should not reach")
		}),
	))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(rec, req)

	// Basic auth is not "Bearer ", should pass through unauthenticated (not API detection via bearer)
	// Note: the request is detected as SSR (Basic auth doesn't trigger API context)
	// Actually with Accept: application/json, it IS detected as API
	// But with Basic auth (not Bearer), authenticateBearer returns error
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_Authenticated(t *testing.T) {
	t.Parallel()

	handler := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "protected")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	ctx := auth.SetUser(req.Context(), &auth.User{ID: 1})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "protected" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "protected")
	}
}

func TestRequireAuth_Unauthenticated_API(t *testing.T) {
	t.Parallel()

	handler := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// Set API context
	ctx := context.WithValue(req.Context(), requestContextKey{}, ContextAPI)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "authentication required" {
		t.Errorf("error = %q, want %q", body["error"], "authentication required")
	}
}

func TestRequireAuth_Unauthenticated_SSR(t *testing.T) {
	t.Parallel()

	handler := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	loc := rec.Header().Get("Location")
	if loc != "/login?next=/dashboard" {
		t.Errorf("Location = %q, want %q", loc, "/login?next=/dashboard")
	}
}

func TestRequireRole_HasRole(t *testing.T) {
	t.Parallel()

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "admin page")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := auth.SetUser(req.Context(), &auth.User{ID: 1, Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	t.Parallel()

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := auth.SetUser(req.Context(), &auth.User{ID: 1, Roles: []string{"viewer"}})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	t.Parallel()

	handler := RequireRole("admin", "editor")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/edit", nil)
	ctx := auth.SetUser(req.Context(), &auth.User{ID: 1, Roles: []string{"editor"}})
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireRole_NoUser(t *testing.T) {
	t.Parallel()

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	// Set API context
	ctx := context.WithValue(req.Context(), requestContextKey{}, ContextAPI)
	req = req.WithContext(ctx)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
