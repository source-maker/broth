package db

import (
	"context"
	"database/sql"
)

// Conn abstracts the common interface between *sql.DB and *sql.Tx,
// enabling transaction-transparent data access.
type Conn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// txContextKey is the context key used to store a transaction.
type txContextKey struct{}

// ConnFromContext returns the transaction stored in ctx if present,
// otherwise returns the provided fallback (typically *sql.DB).
func ConnFromContext(ctx context.Context, fallback Conn) Conn {
	if tx, ok := ctx.Value(txContextKey{}).(Conn); ok {
		return tx
	}
	return fallback
}

// contextWithConn stores a Conn (typically a *sql.Tx) in the context.
func contextWithConn(ctx context.Context, conn Conn) context.Context {
	return context.WithValue(ctx, txContextKey{}, conn)
}
