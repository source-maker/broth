package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/source-maker/broth/session"
)

const (
	csrfTokenLength = 32
	csrfSessionKey  = "_broth_csrf_token"
)

// CSRFConfig configures the CSRF protection middleware.
type CSRFConfig struct {
	// Enabled enables/disables CSRF protection. Default: true.
	Enabled bool

	// HeaderName is the header name for AJAX CSRF token submission.
	// Default: "X-CSRF-Token".
	HeaderName string

	// FieldName is the form field name for CSRF token submission.
	// Default: "_csrf_token".
	FieldName string
}

// csrfContextKey is the context key for the CSRF token.
type csrfContextKey struct{}

// GetCSRFToken retrieves the CSRF token from the context.
func GetCSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(csrfContextKey{}).(string)
	return token
}

func setCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey{}, token)
}

// CSRF returns middleware that protects against Cross-Site Request Forgery.
// Automatically skipped for API context (Bearer token auth is not vulnerable to CSRF).
func CSRF(cfgs ...CSRFConfig) func(http.Handler) http.Handler {
	cfg := CSRFConfig{
		Enabled:    true,
		HeaderName: "X-CSRF-Token",
		FieldName:  "_csrf_token",
	}
	if len(cfgs) > 0 {
		c := cfgs[0]
		cfg.Enabled = c.Enabled
		if c.HeaderName != "" {
			cfg.HeaderName = c.HeaderName
		}
		if c.FieldName != "" {
			cfg.FieldName = c.FieldName
		}
	}

	if !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF for API context
			rc := GetRequestContext(r.Context())
			if rc == ContextAPI {
				next.ServeHTTP(w, r)
				return
			}

			sess := session.FromContext(r.Context())

			// Generate CSRF token if not present in session
			var token string
			if sess != nil {
				token = sess.GetString(csrfSessionKey)
			}
			if token == "" {
				var err error
				token, err = generateCSRFToken()
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if sess != nil {
					sess.Set(csrfSessionKey, token)
				}
			}

			// Store token in context for template access
			ctx := setCSRFToken(r.Context(), token)
			r = r.WithContext(ctx)

			// Safe methods (GET, HEAD, OPTIONS, TRACE) skip validation
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from request (form field or header)
			requestToken := r.FormValue(cfg.FieldName)
			if requestToken == "" {
				requestToken = r.Header.Get(cfg.HeaderName)
			}

			// Constant-time comparison
			if subtle.ConstantTimeCompare([]byte(token), []byte(requestToken)) != 1 {
				http.Error(w, "Forbidden - CSRF token invalid", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func generateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}
