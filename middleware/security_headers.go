package middleware

import (
	"net/http"
	"strconv"
)

// SecurityHeadersConfig configures the security headers middleware.
type SecurityHeadersConfig struct {
	// ContentSecurityPolicy is the CSP header value.
	// If empty, a sensible default is used.
	ContentSecurityPolicy string

	// HSTSEnabled enables the Strict-Transport-Security header.
	// Should be true in production.
	HSTSEnabled bool

	// HSTSMaxAge is the HSTS max-age in seconds. Default: 63072000 (2 years).
	HSTSMaxAge int
}

// SecurityHeaders returns middleware that sets security-related HTTP headers.
// CSP is only applied for SSR context (not needed for JSON API responses).
func SecurityHeaders(cfgs ...SecurityHeadersConfig) func(http.Handler) http.Handler {
	cfg := SecurityHeadersConfig{
		HSTSMaxAge: 63072000,
	}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
		if cfg.HSTSMaxAge == 0 {
			cfg.HSTSMaxAge = 63072000
		}
	}

	csp := cfg.ContentSecurityPolicy
	if csp == "" {
		csp = "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Universal headers
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "0")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			// CSP only for SSR context
			rc := GetRequestContext(r.Context())
			if rc == ContextSSR {
				h.Set("Content-Security-Policy", csp)
			}

			// HSTS (production only)
			if cfg.HSTSEnabled {
				h.Set("Strict-Transport-Security",
					"max-age="+strconv.Itoa(cfg.HSTSMaxAge)+"; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
