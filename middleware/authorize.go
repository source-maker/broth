package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/source-maker/broth/auth"
)

// RequireAuth returns middleware that requires an authenticated user.
// For SSR: redirects to /login. For API: returns 401 JSON.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !auth.IsAuthenticated(r.Context()) {
				rc := GetRequestContext(r.Context())
				switch rc {
				case ContextSSR:
					http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
				case ContextAPI:
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "authentication required",
					})
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns middleware that requires the authenticated user
// to have at least one of the specified roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUser(r.Context())
			if user == nil {
				rc := GetRequestContext(r.Context())
				switch rc {
				case ContextSSR:
					http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
				case ContextAPI:
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "authentication required",
					})
				}
				return
			}

			for _, role := range roles {
				if user.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "insufficient permissions",
			})
		})
	}
}
