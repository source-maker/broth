// Package render provides HTML template rendering and JSON response helpers.
package render

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/source-maker/broth/log"
)

// Config holds template rendering configuration.
type Config struct {
	// TemplateDir is the root directory for templates.
	TemplateDir string

	// LayoutDir is the subdirectory containing layout templates.
	LayoutDir string

	// ComponentDir is the subdirectory containing reusable component templates.
	ComponentDir string

	// Reload enables template reloading on each request (development mode).
	Reload bool

	// FuncMap provides custom template functions.
	FuncMap template.FuncMap
}

// Renderer manages template parsing and rendering.
type Renderer struct {
	cfg    Config
	logger *log.Logger
	tmpls  map[string]*template.Template
	mu     sync.RWMutex
	fs     fs.FS
}

// New creates a Renderer with the given configuration and template filesystem.
func New(cfg Config, fsys fs.FS, logger *log.Logger) *Renderer {
	return &Renderer{
		cfg:    cfg,
		logger: logger,
		tmpls:  make(map[string]*template.Template),
		fs:     fsys,
	}
}

// HTML renders an HTML template with the given data and writes it to w.
// name is the template path relative to the template root (e.g., "account/login.html").
func (re *Renderer) HTML(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, err := re.loadTemplate(name)
	if err != nil {
		re.logger.Error(r.Context(), "render: load template", "name", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		re.logger.Error(r.Context(), "render: execute template", "name", name, "error", err)
	}
}

// JSON writes a JSON response with the given status code and data.
func (re *Renderer) JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		re.logger.Error(nil, "render: encode json", "error", err)
	}
}

// Error renders an error response. For API requests it returns JSON,
// for SSR requests it returns an HTML error page.
func (re *Renderer) Error(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if r.Header.Get("Accept") == "application/json" {
		re.JSON(w, status, map[string]string{"error": msg})
		return
	}
	http.Error(w, msg, status)
}

func (re *Renderer) loadTemplate(name string) (*template.Template, error) {
	if !re.cfg.Reload {
		re.mu.RLock()
		tmpl, ok := re.tmpls[name]
		re.mu.RUnlock()
		if ok {
			return tmpl, nil
		}
	}

	patterns := []string{
		filepath.Join(re.cfg.LayoutDir, "*.html"),
		filepath.Join(re.cfg.ComponentDir, "*.html"),
		name,
	}

	funcMap := template.FuncMap{}
	for k, v := range re.cfg.FuncMap {
		funcMap[k] = v
	}

	tmpl := template.New(filepath.Base(name)).Funcs(funcMap)
	for _, pattern := range patterns {
		matches, err := fs.Glob(re.fs, pattern)
		if err != nil {
			return nil, fmt.Errorf("render: glob %s: %w", pattern, err)
		}
		for _, match := range matches {
			data, err := fs.ReadFile(re.fs, match)
			if err != nil {
				return nil, fmt.Errorf("render: read %s: %w", match, err)
			}
			if _, err := tmpl.New(match).Parse(string(data)); err != nil {
				return nil, fmt.Errorf("render: parse %s: %w", match, err)
			}
		}
	}

	if !re.cfg.Reload {
		re.mu.Lock()
		re.tmpls[name] = tmpl
		re.mu.Unlock()
	}
	return tmpl, nil
}
