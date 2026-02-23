package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrefixPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefix  string
		pattern string
		want    string
	}{
		{
			name:    "GET with prefix",
			prefix:  "/api",
			pattern: "GET /users",
			want:    "GET /api/users",
		},
		{
			name:    "POST with prefix",
			prefix:  "/api",
			pattern: "POST /users",
			want:    "POST /api/users",
		},
		{
			name:    "DELETE with prefix and path param",
			prefix:  "/api/v1",
			pattern: "DELETE /users/{id}",
			want:    "DELETE /api/v1/users/{id}",
		},
		{
			name:    "no method with prefix",
			prefix:  "/api",
			pattern: "/users",
			want:    "/api/users",
		},
		{
			name:    "empty prefix with method",
			prefix:  "",
			pattern: "GET /users",
			want:    "GET /users",
		},
		{
			name:    "empty prefix without method",
			prefix:  "",
			pattern: "/users",
			want:    "/users",
		},
		{
			name:    "PUT with prefix",
			prefix:  "/admin",
			pattern: "PUT /settings/{key}",
			want:    "PUT /admin/settings/{key}",
		},
		{
			name:    "PATCH with prefix",
			prefix:  "/api",
			pattern: "PATCH /articles/{id}",
			want:    "PATCH /api/articles/{id}",
		},
		{
			name:    "OPTIONS with prefix",
			prefix:  "/api",
			pattern: "OPTIONS /cors",
			want:    "OPTIONS /api/cors",
		},
		{
			name:    "HEAD with prefix",
			prefix:  "/api",
			pattern: "HEAD /health",
			want:    "HEAD /api/health",
		},
		{
			name:    "non-method word is treated as path",
			prefix:  "/api",
			pattern: "NOTAMETHOD /foo",
			want:    "/apiNOTAMETHOD /foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := prefixPattern(tt.prefix, tt.pattern)
			if got != tt.want {
				t.Errorf("prefixPattern(%q, %q) = %q, want %q", tt.prefix, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestRouterHandle(t *testing.T) {
	t.Parallel()

	r := New()
	r.Handle(Route{
		Pattern: "GET /hello",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "hello")
		}),
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", string(body), "hello")
	}
}

func TestRouterMount(t *testing.T) {
	t.Parallel()

	r := New()
	routes := []Route{
		{
			Pattern: "GET /users",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, "user list")
			}),
		},
		{
			Pattern: "POST /users",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				io.WriteString(w, "user created")
			}),
		},
		{
			Pattern: "/health",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				io.WriteString(w, "ok")
			}),
		},
	}

	r.Mount("/api", routes)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/users")
	if err != nil {
		t.Fatalf("GET /api/users: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "user list" {
		t.Errorf("GET /api/users body = %q, want %q", string(body), "user list")
	}

	resp, err = http.Post(srv.URL+"/api/users", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/users: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("POST /api/users status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(body) != "user created" {
		t.Errorf("POST /api/users body = %q, want %q", string(body), "user created")
	}

	resp, err = http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Errorf("GET /api/health body = %q, want %q", string(body), "ok")
	}
}

func TestRouterMiddleware(t *testing.T) {
	t.Parallel()

	r := New()

	var order []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "A-before")
			next.ServeHTTP(w, req)
			order = append(order, "A-after")
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "B-before")
			next.ServeHTTP(w, req)
			order = append(order, "B-after")
		})
	})

	r.Handle(Route{
		Pattern: "GET /test",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			order = append(order, "handler")
			io.WriteString(w, "ok")
		}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(rec, req)

	want := []string{"A-before", "B-before", "handler", "B-after", "A-after"}
	if len(order) != len(want) {
		t.Fatalf("middleware order length = %d, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRouterMiddlewareModifiesResponse(t *testing.T) {
	t.Parallel()

	r := New()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Custom", "broth")
			next.ServeHTTP(w, req)
		})
	})

	r.Handle(Route{
		Pattern: "GET /ping",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "pong")
		}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Custom"); got != "broth" {
		t.Errorf("X-Custom header = %q, want %q", got, "broth")
	}
	if rec.Body.String() != "pong" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "pong")
	}
}

func TestRouterServeHTTP(t *testing.T) {
	t.Parallel()

	r := New()
	r.Handle(Route{
		Pattern: "GET /direct",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "direct")
		}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/direct", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "direct" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "direct")
	}
}
