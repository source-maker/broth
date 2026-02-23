package db

import (
	"context"
	"database/sql"
	"testing"
)

// mockConn is a minimal Conn implementation for testing.
// It does not actually execute queries; it only serves to
// verify that ConnFromContext returns the expected Conn.
type mockConn struct {
	name string
}

func (m *mockConn) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, nil
}

func (m *mockConn) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockConn) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	return nil
}

func TestConnFromContext_ReturnsFallbackWhenNoTx(t *testing.T) {
	t.Parallel()

	fallback := &mockConn{name: "fallback"}
	ctx := context.Background()

	got := ConnFromContext(ctx, fallback)
	if got != fallback {
		t.Errorf("ConnFromContext returned %v, want fallback %v", got, fallback)
	}
}

func TestConnFromContext_ReturnsTxFromContext(t *testing.T) {
	t.Parallel()

	fallback := &mockConn{name: "fallback"}
	tx := &mockConn{name: "transaction"}

	// Use the unexported contextWithConn to place a Conn in context.
	ctx := contextWithConn(context.Background(), tx)

	got := ConnFromContext(ctx, fallback)
	if got != tx {
		t.Errorf("ConnFromContext returned %v, want tx %v", got, tx)
	}
}

func TestConnFromContext_TxTakesPrecedenceOverFallback(t *testing.T) {
	t.Parallel()

	fallback := &mockConn{name: "fallback"}
	tx := &mockConn{name: "tx"}

	ctx := contextWithConn(context.Background(), tx)

	got := ConnFromContext(ctx, fallback)

	mc, ok := got.(*mockConn)
	if !ok {
		t.Fatalf("ConnFromContext returned type %T, want *mockConn", got)
	}
	if mc.name != "tx" {
		t.Errorf("ConnFromContext returned conn named %q, want %q", mc.name, "tx")
	}
}

func TestConnFromContext_NilFallback_NoTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	got := ConnFromContext(ctx, nil)
	if got != nil {
		t.Errorf("ConnFromContext with nil fallback and no tx = %v, want nil", got)
	}
}

func TestContextWithConn_RoundTrip(t *testing.T) {
	t.Parallel()

	conn := &mockConn{name: "roundtrip"}
	ctx := contextWithConn(context.Background(), conn)

	// Verify via ConnFromContext with a different fallback.
	other := &mockConn{name: "other"}
	got := ConnFromContext(ctx, other)

	mc, ok := got.(*mockConn)
	if !ok {
		t.Fatalf("expected *mockConn, got %T", got)
	}
	if mc.name != "roundtrip" {
		t.Errorf("got conn named %q, want %q", mc.name, "roundtrip")
	}
}
