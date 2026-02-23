package auth

import "context"

// Action represents an operation on a resource.
type Action string

const (
	ActionView   Action = "view"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Policy defines authorization rules for a resource of type T.
type Policy[T any] interface {
	Authorize(ctx context.Context, user *User, action Action, resource T) error
}
