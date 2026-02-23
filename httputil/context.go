// Package httputil provides type-safe context key utilities using generics.
package httputil

import "context"

// CtxKey is a generic, type-safe context key.
// Each distinct CtxKey value is unique by its name and type parameter.
type CtxKey[T any] struct{ Name string }

// Get retrieves a value from the context by its typed key.
func Get[T any](ctx context.Context, key CtxKey[T]) (T, bool) {
	val, ok := ctx.Value(key).(T)
	return val, ok
}

// Set stores a value in the context by its typed key.
func Set[T any](ctx context.Context, key CtxKey[T], val T) context.Context {
	return context.WithValue(ctx, key, val)
}

// RequestID is the context key for the request ID.
var RequestID = CtxKey[string]{Name: "request_id"}
