package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"testing"

	"github.com/source-maker/broth/log"
	"github.com/stephenafamo/bob"
)

// --- minimal SQL driver for in-process testing ---
// stubDriver and its helpers satisfy database/sql/driver so that sql.Open()
// succeeds and sql.DB.Ping() can complete without a real DB server.

func init() {
	sql.Register("stub", &stubDriver{})
}

type stubDriver struct{}

func (d *stubDriver) Open(_ string) (driver.Conn, error) { return &stubConn{}, nil }

type stubConn struct{}

func (c *stubConn) Prepare(query string) (driver.Stmt, error) {
	return &stubStmt{}, nil
}
func (c *stubConn) Close() error                         { return nil }
func (c *stubConn) Begin() (driver.Tx, error)            { return &stubTx{}, nil }
func (c *stubConn) Exec(_ string, _ []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

// Ping satisfies the driver.Pinger interface; database/sql calls it for Ping().
func (c *stubConn) Ping(_ context.Context) error { return nil }

type stubStmt struct{}

func (s *stubStmt) Close() error                                       { return nil }
func (s *stubStmt) NumInput() int                                      { return 0 }
func (s *stubStmt) Exec(_ []driver.Value) (driver.Result, error)      { return driver.RowsAffected(0), nil }
func (s *stubStmt) Query(_ []driver.Value) (driver.Rows, error)       { return &stubRows{}, nil }

type stubRows struct{ done bool }

func (r *stubRows) Columns() []string              { return []string{} }
func (r *stubRows) Close() error                   { return nil }
func (r *stubRows) Next(_ []driver.Value) error {
	if r.done {
		return fmt.Errorf("EOF")
	}
	r.done = true
	return driver.ErrSkip
}

type stubTx struct{}

func (t *stubTx) Commit() error   { return nil }
func (t *stubTx) Rollback() error { return nil }

// newTestDatabase creates a Database backed by the in-process stub driver.
func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	sqlDB, err := sql.Open("stub", "stub://test")
	if err != nil {
		t.Fatalf("sql.Open(stub): %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	logger := log.New(slog.LevelDebug)
	return &Database{db: sqlDB, logger: logger}
}

// --- BobDB tests ---

// TestBobDB_ReturnsNonNil verifies that BobDB() returns a usable bob.DB value.
func TestBobDB_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	d := newTestDatabase(t)
	bobDB := d.BobDB()

	// bob.DB embeds *sql.DB; a zero-value bob.DB has a nil inner pointer.
	// We verify the returned value wraps the actual *sql.DB.
	if bobDB.DB == nil {
		t.Fatal("BobDB().DB is nil; expected non-nil *sql.DB")
	}
}

// TestBobDB_ImplementsExecutor verifies that the value returned by BobDB()
// satisfies the bob.Executor interface at compile time.
func TestBobDB_ImplementsExecutor(t *testing.T) {
	t.Parallel()

	d := newTestDatabase(t)
	var _ bob.Executor = d.BobDB() // compile-time assertion
}

// TestBobDB_SameUnderlyingDB verifies that BobDB wraps the exact *sql.DB
// stored in Database, not a copy.
func TestBobDB_SameUnderlyingDB(t *testing.T) {
	t.Parallel()

	d := newTestDatabase(t)
	bobDB := d.BobDB()

	if bobDB.DB != d.db {
		t.Error("BobDB().DB pointer differs from Database.db; expected same instance")
	}
}

// --- ExecutorFromContext tests ---

// bobFallback is a minimal bob.Executor for use as the fallback parameter.
// We use the real bob.DB here because bob.Executor is a concrete interface
// and we need a legitimate implementation.
func newFallbackExecutor(t *testing.T) bob.Executor {
	t.Helper()
	sqlDB, err := sql.Open("stub", "stub://fallback")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return bob.NewDB(sqlDB)
}

// TestExecutorFromContext_NoTx verifies that ExecutorFromContext returns the
// fallback executor when the context carries no transaction.
func TestExecutorFromContext_NoTx(t *testing.T) {
	t.Parallel()

	fallback := newFallbackExecutor(t)
	ctx := context.Background()

	got := ExecutorFromContext(ctx, fallback)
	if got != fallback {
		t.Errorf("ExecutorFromContext(no-tx) = %T, want fallback %T", got, fallback)
	}
}

// TestExecutorFromContext_WithTx verifies that ExecutorFromContext returns a
// bob.Tx (not the fallback) when a *sql.Tx is present in the context.
//
// We use contextWithConn (internal helper, same package) to inject the tx
// without starting a real database transaction.
func TestExecutorFromContext_WithTx(t *testing.T) {
	t.Parallel()

	fallback := newFallbackExecutor(t)

	// Open a stub DB and begin a transaction.
	sqlDB, err := sql.Open("stub", "stub://tx-test")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer sqlDB.Close()

	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("sql.DB.Begin: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Store the *sql.Tx in the context via the unexported helper.
	ctx := contextWithConn(context.Background(), tx)

	got := ExecutorFromContext(ctx, fallback)

	// The result must NOT be the fallback — it should be a bob.Tx.
	if got == fallback {
		t.Fatal("ExecutorFromContext(with-tx) returned fallback; expected bob.Tx")
	}

	// bob.Tx is a concrete struct; assert the returned executor is a bob.Tx.
	if _, ok := got.(bob.Tx); !ok {
		t.Errorf("ExecutorFromContext(with-tx) returned %T; expected bob.Tx", got)
	}
}

// TestExecutorFromContext_WrongTypeInContext verifies that ExecutorFromContext
// falls back when the context value exists but is not a *sql.Tx.
func TestExecutorFromContext_WrongTypeInContext(t *testing.T) {
	t.Parallel()

	fallback := newFallbackExecutor(t)

	// Store an unexpected type (mockConn, not *sql.Tx) in the context using
	// the same key that ExecutorFromContext reads.
	ctx := contextWithConn(context.Background(), &mockConn{name: "wrong-type"})

	got := ExecutorFromContext(ctx, fallback)
	if got != fallback {
		t.Errorf("ExecutorFromContext(wrong-type) = %T, want fallback", got)
	}
}

// TestExecutorFromContext_NilFallback_NoTx verifies that ExecutorFromContext
// returns nil when no tx is present and fallback is nil.
func TestExecutorFromContext_NilFallback_NoTx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := ExecutorFromContext(ctx, nil)
	if got != nil {
		t.Errorf("ExecutorFromContext(nil-fallback, no-tx) = %v, want nil", got)
	}
}

// TestExecutorFromContext_ContextWithValue_NonTxKey verifies that an
// unrelated context value does not affect the executor selection.
func TestExecutorFromContext_ContextWithValue_NonTxKey(t *testing.T) {
	t.Parallel()

	type otherKey struct{}
	fallback := newFallbackExecutor(t)

	// Inject a value with a different key — should not affect behavior.
	ctx := context.WithValue(context.Background(), otherKey{}, "unrelated")

	got := ExecutorFromContext(ctx, fallback)
	if got != fallback {
		t.Errorf("ExecutorFromContext(unrelated-key) = %T, want fallback", got)
	}
}
