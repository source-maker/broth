package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/session"
)

// AuthConfig configures the authentication middleware.
type AuthConfig struct {
	// TokenService validates JWT access tokens (required for API auth).
	TokenService *auth.TokenService

	// UserFinder loads a user by ID (required for session-based auth).
	// Returns nil, nil if the user is not found.
	UserFinder func(ctx interface{ Value(any) any }, id int64) (*auth.User, error)
}

// Auth returns middleware that authenticates users based on request context.
// For API requests: validates Bearer token from Authorization header.
// For SSR requests: loads user from session.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := GetRequestContext(r.Context())

			var user *auth.User
			var err error

			switch rc {
			case ContextAPI:
				user, err = authenticateBearer(r, cfg)
			case ContextSSR:
				user, err = authenticateSession(r, cfg)
			}

			if err != nil {
				handleAuthError(w, r, rc, err)
				return
			}

			if user != nil {
				ctx := auth.SetUser(r.Context(), user)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authenticateBearer(r *http.Request, cfg AuthConfig) (*auth.User, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, nil // Unauthenticated (not an error)
	}

	if !strings.HasPrefix(header, "Bearer ") {
		return nil, errors.New("auth: invalid authorization header format")
	}

	if cfg.TokenService == nil {
		return nil, errors.New("auth: token service not configured")
	}

	token := strings.TrimPrefix(header, "Bearer ")
	claims, err := cfg.TokenService.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	return &auth.User{
		ID:    claims.UserID,
		Email: claims.Email,
		Roles: claims.Roles,
	}, nil
}

func authenticateSession(r *http.Request, cfg AuthConfig) (*auth.User, error) {
	sess := session.FromContext(r.Context())
	if sess == nil {
		return nil, nil
	}

	userIDVal, ok := sess.Data["user_id"]
	if !ok {
		return nil, nil
	}

	var userID int64
	switch v := userIDVal.(type) {
	case int64:
		userID = v
	case float64:
		userID = int64(v)
	case int:
		userID = int64(v)
	default:
		return nil, nil
	}

	if userID == 0 {
		return nil, nil
	}

	if cfg.UserFinder == nil {
		return nil, nil
	}

	return cfg.UserFinder(r.Context(), userID)
}

func handleAuthError(w http.ResponseWriter, r *http.Request, rc RequestContext, err error) {
	switch rc {
	case ContextAPI:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"` + err.Error() + `"}`))
	case ContextSSR:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
