package middleware

import (
	"net/http"
	"time"

	"github.com/source-maker/broth/session"
)

// SessionConfig configures the session middleware.
type SessionConfig struct {
	// CookieName is the name of the session cookie. Default: "_broth_session".
	CookieName string

	// MaxAge is the default session max-age in seconds. 0 means browser session.
	MaxAge int

	// Secure sets the Secure flag on the cookie (HTTPS only).
	Secure bool

	// SameSite sets the SameSite attribute. Default: http.SameSiteLaxMode.
	SameSite http.SameSite

	// Path sets the cookie path. Default: "/".
	Path string
}

// Session returns middleware that manages cookie-based sessions.
// It loads the session from the cookie at the start of the request,
// stores it in the context, and saves it back to the cookie after the handler.
func Session(store *session.CookieStore, cfgs ...SessionConfig) func(http.Handler) http.Handler {
	cfg := SessionConfig{
		CookieName: "_broth_session",
		SameSite:   http.SameSiteLaxMode,
		Path:       "/",
	}
	if len(cfgs) > 0 {
		c := cfgs[0]
		if c.CookieName != "" {
			cfg.CookieName = c.CookieName
		}
		cfg.MaxAge = c.MaxAge
		cfg.Secure = c.Secure
		if c.SameSite != 0 {
			cfg.SameSite = c.SameSite
		}
		if c.Path != "" {
			cfg.Path = c.Path
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Load session from cookie
			sess := loadSession(r, store, cfg.CookieName)

			// Store in context
			ctx := session.NewContext(r.Context(), sess)
			r = r.WithContext(ctx)

			// Wrap the response writer to intercept WriteHeader
			sw := &sessionWriter{
				ResponseWriter: w,
				sess:           sess,
				store:          store,
				cfg:            cfg,
			}

			// Call the next handler
			next.ServeHTTP(sw, r)

			// If handler didn't call WriteHeader/Write, save now
			if !sw.headerWritten {
				sw.saveSession()
			}
		})
	}
}

// sessionWriter wraps http.ResponseWriter to set session cookies
// before the response headers are flushed.
type sessionWriter struct {
	http.ResponseWriter
	sess          *session.Session
	store         *session.CookieStore
	cfg           SessionConfig
	headerWritten bool
}

func (sw *sessionWriter) WriteHeader(code int) {
	if !sw.headerWritten {
		sw.saveSession()
		sw.headerWritten = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Write(b []byte) (int, error) {
	if !sw.headerWritten {
		sw.saveSession()
		sw.headerWritten = true
	}
	return sw.ResponseWriter.Write(b)
}

func (sw *sessionWriter) saveSession() {
	saveSession(sw.ResponseWriter, sw.sess, sw.store, sw.cfg)
}

func loadSession(r *http.Request, store *session.CookieStore, cookieName string) *session.Session {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return newSession()
	}

	sess, err := store.Decode(cookie.Value)
	if err != nil {
		return newSession()
	}

	return sess
}

func newSession() *session.Session {
	return &session.Session{
		ID:        session.GenerateID(),
		Data:      make(map[string]any),
		CreatedAt: time.Now(),
		IsNew:     true,
	}
}

func saveSession(w http.ResponseWriter, sess *session.Session, store *session.CookieStore, cfg SessionConfig) {
	if sess.IsDestroyed() {
		// Clear the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.CookieName,
			Value:    "",
			Path:     cfg.Path,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   cfg.Secure,
			SameSite: cfg.SameSite,
		})
		return
	}

	if !sess.IsNew && !sess.IsModified() {
		return // No changes, skip saving
	}

	encoded, err := store.Encode(sess)
	if err != nil {
		return // Silently fail; the logger middleware should catch issues
	}

	maxAge := cfg.MaxAge
	if sess.MaxAge() > 0 {
		maxAge = sess.MaxAge()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    encoded,
		Path:     cfg.Path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}
