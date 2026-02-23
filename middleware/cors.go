package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is the list of allowed origins.
	// Use "*" to allow all origins (not recommended for production with credentials).
	AllowedOrigins []string

	// AllowedMethods is the list of allowed HTTP methods.
	// Default: GET, POST, PUT, PATCH, DELETE.
	AllowedMethods []string

	// AllowedHeaders is the list of allowed request headers.
	// Default: Content-Type, Authorization, X-CSRF-Token.
	AllowedHeaders []string

	// AllowCredentials indicates whether credentials (cookies, auth headers) are allowed.
	AllowCredentials bool

	// MaxAge is the max-age for preflight cache in seconds. Default: 86400 (24h).
	MaxAge int
}

// CORS returns middleware that handles Cross-Origin Resource Sharing.
// Only applies to API context (SSR same-origin requests don't need CORS).
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Content-Type", "Authorization", "X-CSRF-Token"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 86400
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := GetRequestContext(r.Context())

			// Skip CORS for SSR context
			if rc == ContextSSR {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check allowed origin
			if isAllowedOrigin(origin, cfg.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")

				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			// Preflight request
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods",
					strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers",
					strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age",
					strconv.Itoa(cfg.MaxAge))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}
