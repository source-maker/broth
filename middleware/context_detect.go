package middleware

import (
	"context"
	"net/http"
	"strings"
)

// RequestContext represents the request context (SSR or API).
type RequestContext int

const (
	// ContextSSR indicates a browser-based SSR request.
	ContextSSR RequestContext = iota
	// ContextAPI indicates an API client request.
	ContextAPI
)

// String returns the string representation of RequestContext.
func (rc RequestContext) String() string {
	switch rc {
	case ContextAPI:
		return "api"
	default:
		return "ssr"
	}
}

// requestContextKey is the context key for storing RequestContext.
type requestContextKey struct{}

// DetectRequestContext determines SSR/API from request headers.
// Detection rules (priority order):
//  1. Content-Type: application/json → API
//  2. Accept: application/json (without text/html) → API
//  3. Authorization: Bearer → API
//  4. X-Requested-With: XMLHttpRequest → API
//  5. None of the above → SSR (safe default)
func DetectRequestContext(r *http.Request) RequestContext {
	// 1. Content-Type check
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		return ContextAPI
	}

	// 2. Accept check (if text/html is also present, prefer SSR)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") &&
		!strings.Contains(accept, "text/html") {
		return ContextAPI
	}

	// 3. Authorization: Bearer check
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return ContextAPI
	}

	// 4. X-Requested-With check (AJAX)
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return ContextAPI
	}

	// 5. Default to SSR (safe side)
	return ContextSSR
}

// GetRequestContext retrieves RequestContext from the context.
// Returns ContextSSR if not set (safe default).
func GetRequestContext(ctx context.Context) RequestContext {
	if rc, ok := ctx.Value(requestContextKey{}).(RequestContext); ok {
		return rc
	}
	return ContextSSR
}

// ContextDetect returns middleware that detects SSR/API context
// and stores it in the request context for downstream middleware.
func ContextDetect() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := DetectRequestContext(r)
			ctx := context.WithValue(r.Context(), requestContextKey{}, rc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ForceContext overrides the detected context for a specific handler.
func ForceContext(rc RequestContext, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestContextKey{}, rc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
