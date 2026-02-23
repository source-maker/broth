// Package auth provides authentication utilities for Broth applications.
package auth

import "context"

// User represents an authenticated user in the request context.
type User struct {
	ID    int64
	Email string
	Name  string
	Roles []string
}

// userContextKey is the context key for the authenticated user.
type userContextKey struct{}

// SetUser stores the authenticated user in the context.
func SetUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// GetUser retrieves the authenticated user from the context.
// Returns nil if no user is set.
func GetUser(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey{}).(*User)
	return u
}

// MustGetUser retrieves the authenticated user from the context.
// Panics if no user is set.
func MustGetUser(ctx context.Context) *User {
	u := GetUser(ctx)
	if u == nil {
		panic("auth: no user in context")
	}
	return u
}

// IsAuthenticated returns true if a user is present in the context.
func IsAuthenticated(ctx context.Context) bool {
	return GetUser(ctx) != nil
}

// HasRole returns true if the user has the given role.
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}
