package render

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/source-maker/broth/log"
)

func testLogger() *log.Logger {
	return log.New(slog.LevelDebug)
}

func TestJSON(t *testing.T) {
	t.Parallel()

	re := New(Config{}, fstest.MapFS{}, testLogger())

	rec := httptest.NewRecorder()
	re.JSON(rec, http.StatusOK, map[string]string{"message": "hello"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body["message"] != "hello" {
		t.Errorf("body[message] = %q, want %q", body["message"], "hello")
	}
}

func TestJSON_CustomStatus(t *testing.T) {
	t.Parallel()

	re := New(Config{}, fstest.MapFS{}, testLogger())

	rec := httptest.NewRecorder()
	re.JSON(rec, http.StatusCreated, map[string]int{"id": 42})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestError_JSON(t *testing.T) {
	t.Parallel()

	re := New(Config{}, fstest.MapFS{}, testLogger())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json")

	re.Error(rec, req, http.StatusNotFound, "not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body["error"] != "not found" {
		t.Errorf("body[error] = %q, want %q", body["error"], "not found")
	}
}

func TestError_HTML(t *testing.T) {
	t.Parallel()

	re := New(Config{}, fstest.MapFS{}, testLogger())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")

	re.Error(rec, req, http.StatusBadRequest, "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "bad request") {
		t.Errorf("body should contain 'bad request', got %q", rec.Body.String())
	}
}

func TestHTML_WithDefineBlock(t *testing.T) {
	t.Parallel()

	// The renderer creates template.New(filepath.Base(name)), then parses files
	// via tmpl.New(match).Parse(). For the content to render, templates must
	// use {{define}} blocks matching filepath.Base(name).
	fs := fstest.MapFS{
		"account/hello.html": &fstest.MapFile{
			Data: []byte(`{{define "hello.html"}}<h1>Hello {{.Name}}</h1>{{end}}`),
		},
	}

	re := New(Config{
		TemplateDir:  ".",
		LayoutDir:    "layouts",
		ComponentDir: "components",
	}, fs, testLogger())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	re.HTML(rec, req, "account/hello.html", map[string]string{"Name": "World"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	if !strings.Contains(rec.Body.String(), "Hello World") {
		t.Errorf("body should contain 'Hello World', got %q", rec.Body.String())
	}
}

func TestHTML_CachesTemplate(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"pages/cached.html": &fstest.MapFile{
			Data: []byte(`{{define "cached.html"}}<p>cached</p>{{end}}`),
		},
	}

	re := New(Config{
		Reload: false,
	}, fs, testLogger())

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	re.HTML(rec1, req1, "pages/cached.html", nil)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	re.HTML(rec2, req2, "pages/cached.html", nil)

	if rec2.Code != http.StatusOK {
		t.Errorf("second render status = %d, want %d", rec2.Code, http.StatusOK)
	}
	if !strings.Contains(rec2.Body.String(), "cached") {
		t.Errorf("body should contain 'cached', got %q", rec2.Body.String())
	}
}

func TestNew_CreatesRenderer(t *testing.T) {
	t.Parallel()

	re := New(Config{Reload: true}, fstest.MapFS{}, testLogger())
	if re == nil {
		t.Fatal("New returned nil")
	}
}
