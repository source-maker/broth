package broth_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/source-maker/broth"
	"github.com/source-maker/broth/log"
	"github.com/source-maker/broth/router"
)

// stubModule implements broth.Module for testing.
type stubModule struct {
	name   string
	routes []router.Route
}

func (m *stubModule) Name() string          { return m.name }
func (m *stubModule) Routes() []router.Route { return m.routes }

// lifecycleModule implements Module, OnStart, and OnShutdown.
type lifecycleModule struct {
	name         string
	routes       []router.Route
	started      bool
	shutdown     bool
	startErr     error
	shutdownErr  error
}

func (m *lifecycleModule) Name() string          { return m.name }
func (m *lifecycleModule) Routes() []router.Route { return m.routes }

func (m *lifecycleModule) Start(_ context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *lifecycleModule) Shutdown(_ context.Context) error {
	if m.shutdownErr != nil {
		return m.shutdownErr
	}
	m.shutdown = true
	return nil
}

func newTestLogger() *log.Logger {
	return log.New(slog.LevelDebug)
}

func TestNew(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())
	if app == nil {
		t.Fatal("New returned nil")
	}
	if mods := app.Modules(); len(mods) != 0 {
		t.Errorf("new app has %d modules, want 0", len(mods))
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	mod1 := &stubModule{name: "account"}
	mod2 := &stubModule{name: "article"}

	app.Register(mod1, mod2)

	mods := app.Modules()
	if len(mods) != 2 {
		t.Fatalf("Modules() len = %d, want 2", len(mods))
	}
	if mods[0].Name() != "account" {
		t.Errorf("Modules()[0].Name() = %q, want %q", mods[0].Name(), "account")
	}
	if mods[1].Name() != "article" {
		t.Errorf("Modules()[1].Name() = %q, want %q", mods[1].Name(), "article")
	}
}

func TestRegister_Incremental(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	app.Register(&stubModule{name: "first"})
	app.Register(&stubModule{name: "second"})

	if len(app.Modules()) != 2 {
		t.Errorf("Modules() len = %d, want 2", len(app.Modules()))
	}
}

func TestMountAndHandler(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	mod := &stubModule{
		name: "health",
		routes: []router.Route{
			{
				Pattern: "GET /ping",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("pong"))
				}),
			},
		},
	}

	app.Register(mod)
	app.Mount()

	handler := app.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	// The route should be mounted at /health/ping.
	req := httptest.NewRequest(http.MethodGet, "/health/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health/ping status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "pong" {
		t.Errorf("GET /health/ping body = %q, want %q", body, "pong")
	}
}

func TestMount_MultipleModules(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	modA := &stubModule{
		name: "alpha",
		routes: []router.Route{
			{
				Pattern: "GET /info",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("alpha"))
				}),
			},
		},
	}
	modB := &stubModule{
		name: "beta",
		routes: []router.Route{
			{
				Pattern: "GET /info",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("beta"))
				}),
			},
		},
	}

	app.Register(modA, modB)
	app.Mount()

	handler := app.Handler()

	tests := []struct {
		path string
		want string
	}{
		{"/alpha/info", "alpha"},
		{"/beta/info", "beta"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if body := rec.Body.String(); body != tt.want {
				t.Errorf("GET %s body = %q, want %q", tt.path, body, tt.want)
			}
		})
	}
}

func TestUse_Middleware(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	// Middleware that adds a custom header.
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Broth", "active")
			next.ServeHTTP(w, r)
		})
	})

	mod := &stubModule{
		name: "test",
		routes: []router.Route{
			{
				Pattern: "GET /hello",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("world"))
				}),
			},
		},
	}
	app.Register(mod)
	app.Mount()

	req := httptest.NewRequest(http.MethodGet, "/test/hello", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Broth"); got != "active" {
		t.Errorf("X-Broth header = %q, want %q", got, "active")
	}
}

func TestStart(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	lm := &lifecycleModule{name: "starter"}
	plain := &stubModule{name: "plain"}

	app.Register(lm, plain)

	err := app.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if !lm.started {
		t.Error("lifecycle module was not started")
	}
}

func TestStart_Error(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	lm := &lifecycleModule{
		name:     "broken",
		startErr: errors.New("init failed"),
	}
	app.Register(lm)

	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want error")
	}
	if got := err.Error(); got != "broth: start broken: init failed" {
		t.Errorf("Start() error = %q, want %q", got, "broth: start broken: init failed")
	}
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	lm := &lifecycleModule{name: "stopper"}
	plain := &stubModule{name: "plain"}

	app.Register(lm, plain)

	err := app.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown() error = %v, want nil", err)
	}
	if !lm.shutdown {
		t.Error("lifecycle module was not shut down")
	}
}

func TestShutdown_Error(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	lm := &lifecycleModule{
		name:        "broken",
		shutdownErr: errors.New("cleanup failed"),
	}
	app.Register(lm)

	err := app.Shutdown(context.Background())
	if err == nil {
		t.Fatal("Shutdown() error = nil, want error")
	}
	if got := err.Error(); got != "broth: shutdown broken: cleanup failed" {
		t.Errorf("Shutdown() error = %q, want %q", got, "broth: shutdown broken: cleanup failed")
	}
}

func TestShutdown_ContinuesOnError(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	broken := &lifecycleModule{
		name:        "broken",
		shutdownErr: errors.New("fail"),
	}
	healthy := &lifecycleModule{name: "healthy"}

	app.Register(broken, healthy)

	err := app.Shutdown(context.Background())
	// Should return the first error but still call Shutdown on all modules.
	if err == nil {
		t.Fatal("Shutdown() error = nil, want error")
	}
	if !healthy.shutdown {
		t.Error("healthy module was not shut down despite broken module error")
	}
}

func TestStartAndShutdown_FullLifecycle(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())

	lm := &lifecycleModule{name: "full"}
	app.Register(lm)

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !lm.started {
		t.Error("module not started")
	}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !lm.shutdown {
		t.Error("module not shut down")
	}
}

func TestHandler_NoModules(t *testing.T) {
	t.Parallel()

	app := broth.New(newTestLogger())
	app.Mount()

	handler := app.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil with no modules")
	}

	// Request to unknown path should get 404.
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
