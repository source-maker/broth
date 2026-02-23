// Package router provides an http.ServeMux wrapper with middleware support.
package router

import (
	"net/http"
	"strings"
)

// Middleware is a standard HTTP middleware function.
type Middleware = func(http.Handler) http.Handler

// Route describes a single route with its pattern and handler.
type Route struct {
	// Pattern uses Go 1.22+ syntax, e.g. "GET /users/{id}".
	Pattern string
	Handler http.Handler
}

// Router wraps http.ServeMux and adds middleware chain support.
type Router struct {
	mux        *http.ServeMux
	middleware []Middleware
}

// New creates a new Router.
func New() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// Use appends middleware to the router's chain.
func (r *Router) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// Handle registers a route with the router.
func (r *Router) Handle(route Route) {
	r.mux.Handle(route.Pattern, route.Handler)
}

// Mount registers all routes under a path prefix.
// Each route pattern is prefixed with the given prefix.
// Patterns use Go 1.22+ syntax (e.g. "GET /users/{id}"), so the prefix
// is inserted between the method and the path portion.
func (r *Router) Mount(prefix string, routes []Route) {
	for _, route := range routes {
		pattern := prefixPattern(prefix, route.Pattern)
		r.mux.Handle(pattern, route.Handler)
	}
}

// prefixPattern inserts prefix between the HTTP method and path in a
// Go 1.22+ ServeMux pattern. For example:
//
//	prefixPattern("/api", "GET /users") => "GET /api/users"
//	prefixPattern("/api", "/users")     => "/api/users"
func prefixPattern(prefix, pattern string) string {
	// Go 1.22+ patterns may start with "METHOD " (e.g. "GET /path").
	if method, path, ok := strings.Cut(pattern, " "); ok && isHTTPMethod(method) {
		return method + " " + prefix + path
	}
	// No method prefix -- just concatenate.
	return prefix + pattern
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		return true
	}
	return false
}

// Handler returns the final http.Handler with all middleware applied.
func (r *Router) Handler() http.Handler {
	var h http.Handler = r.mux
	// Apply middleware in reverse order so the first Use'd middleware is outermost.
	for i := len(r.middleware) - 1; i >= 0; i-- {
		h = r.middleware[i](h)
	}
	return h
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Handler().ServeHTTP(w, req)
}
