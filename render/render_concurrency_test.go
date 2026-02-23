package render_test

import (
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/source-maker/broth/log"
	"github.com/source-maker/broth/render"
)

func TestRenderer_Concurrency(t *testing.T) {
	// Setup a mock filesystem. loadTemplate creates the root template via
	// template.New(filepath.Base(name)) and parses files as associated
	// templates via tmpl.New(match).Parse(...). The template file path must
	// differ from filepath.Base(name) (i.e., use a subdirectory) so that
	// tmpl.New(match) creates a distinct associated template. Templates use
	// {{define "basename"}} blocks to populate the root template name.
	fsys := fstest.MapFS{
		"pages/page.html": &fstest.MapFile{
			Data: []byte(`{{define "page.html"}}Hello, World!{{end}}`),
		},
	}

	cfg := render.Config{
		TemplateDir:  ".",
		LayoutDir:    "layouts",
		ComponentDir: "components",
		Reload:       false, // Production mode (caching enabled)
	}

	logger := log.New(slog.LevelError)
	r := render.New(cfg, fsys, logger)

	// Simulate concurrent requests
	var wg sync.WaitGroup
	workers := 20
	requests := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requests; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/", nil)
				r.HTML(w, req, "pages/page.html", nil)

				if w.Code != 200 {
					t.Errorf("request failed: code=%d", w.Code)
				}
				if body := w.Body.String(); body != "Hello, World!" {
					t.Errorf("body = %q, want %q", body, "Hello, World!")
				}
			}
		}()
	}

	wg.Wait()
}
