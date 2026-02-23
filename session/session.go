// Package session provides HTTP session management.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"
)

// Store defines the interface for session persistence.
type Store interface {
	Get(ctx context.Context, id string) (*Session, error)
	Save(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
	GC(ctx context.Context) error
}

// Session represents a user session.
type Session struct {
	ID        string
	Data      map[string]any
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	IsNew     bool

	// Internal state for middleware
	destroyed   bool
	regenerated bool
	oldID       string
	modified    bool
	maxAge      int // seconds; 0 means browser session
}

// sessionContextKey is the context key used to store a session.
type sessionContextKey struct{}

// FromContext retrieves the session from the context.
func FromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey{}).(*Session)
	return s
}

// NewContext stores a session in the context.
func NewContext(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

// GetString retrieves a string value from session data.
func (s *Session) GetString(key string) string {
	if s.Data == nil {
		return ""
	}
	v, _ := s.Data[key].(string)
	return v
}

// Set stores a value in session data.
func (s *Session) Set(key string, value any) {
	if s.Data == nil {
		s.Data = make(map[string]any)
	}
	s.Data[key] = value
	s.modified = true
}

// Regenerate invalidates the current session ID and generates a new one.
// This should be called on login to prevent session fixation attacks.
func (s *Session) Regenerate() {
	s.oldID = s.ID
	s.ID = GenerateID()
	s.regenerated = true
	s.modified = true
}

// Destroy marks the session for deletion. The middleware will clear the cookie
// and remove server-side data if applicable.
func (s *Session) Destroy() {
	s.destroyed = true
	s.modified = true
}

// SetMaxAge sets the session expiration in seconds.
// Use for "Remember Me" functionality (e.g., 30 days = 2592000).
func (s *Session) SetMaxAge(seconds int) {
	s.maxAge = seconds
	s.ExpiresAt = time.Now().Add(time.Duration(seconds) * time.Second)
	s.modified = true
}

// IsDestroyed returns whether the session has been marked for destruction.
func (s *Session) IsDestroyed() bool {
	return s.destroyed
}

// IsRegenerated returns whether the session ID has been regenerated.
func (s *Session) IsRegenerated() bool {
	return s.regenerated
}

// OldID returns the previous session ID after regeneration.
func (s *Session) OldID() string {
	return s.oldID
}

// IsModified returns whether the session data has been changed.
func (s *Session) IsModified() bool {
	return s.modified
}

// MaxAge returns the session max-age in seconds.
func (s *Session) MaxAge() int {
	return s.maxAge
}

// GenerateID creates a cryptographically secure session ID.
func GenerateID() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("session: failed to generate ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}
