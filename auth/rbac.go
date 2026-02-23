package auth

// Role represents a role assigned to a user.
type Role struct {
	Name        string
	Permissions []Permission
}

// Permission represents an authorization for an action.
// Format: "resource:action" (e.g., "article:create", "user:delete").
type Permission string

// Standard role names.
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// Standard permissions.
const (
	PermCreate Permission = "create"
	PermRead   Permission = "read"
	PermUpdate Permission = "update"
	PermDelete Permission = "delete"
)
