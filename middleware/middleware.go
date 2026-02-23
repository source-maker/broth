// Package middleware provides standard HTTP middleware for Broth applications.
package middleware

import "net/http"

// Chain composes multiple middleware into a single middleware.
// Middleware are applied in order: the first argument is the outermost.
func Chain(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			final = mw[i](final)
		}
		return final
	}
}
