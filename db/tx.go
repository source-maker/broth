package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/source-maker/broth/log"
)

// TxManager defines the interface for running functions within a transaction.
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
	RunInTxWithOpts(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error
}

// globalSpID is a process-wide counter for savepoint names. Using a global
// counter (instead of per-TxMgr) ensures that savepoint names are unique even
// when multiple TxMgr instances coexist, which aids debugging and log analysis.
var globalSpID atomic.Int64

// TxMgr implements TxManager with nested transaction support via SAVEPOINTs.
type TxMgr struct {
	db     *sql.DB
	logger *log.Logger
}

// NewTxManager creates a new TxMgr.
func NewTxManager(db *sql.DB, logger *log.Logger) *TxMgr {
	return &TxMgr{db: db, logger: logger}
}

// RunInTx executes fn within a transaction using default options.
func (m *TxMgr) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.RunInTxWithOpts(ctx, nil, fn)
}

// RunInTxWithOpts executes fn within a transaction.
// If a transaction already exists in ctx, a SAVEPOINT is used for nesting.
func (m *TxMgr) RunInTxWithOpts(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
	// Check for existing transaction (nested case).
	// Use a non-nil check on the context value to avoid a fragile concrete
	// type assertion; contextWithConn stores a Conn interface value.
	if ctx.Value(txContextKey{}) != nil {
		return m.runSavepoint(ctx, fn)
	}
	return m.runNewTx(ctx, opts, fn)
}

func (m *TxMgr) runNewTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
	tx, err := m.db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	txCtx := contextWithConn(ctx, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			m.logger.Error(ctx, "db: rollback failed", "error", rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

func (m *TxMgr) runSavepoint(ctx context.Context, fn func(ctx context.Context) error) error {
	// Extract the Conn stored by contextWithConn. Within a transaction this
	// is always a *sql.Tx stored via runNewTx, so the ExecContext calls
	// below execute on the correct underlying transaction.
	conn, ok := ctx.Value(txContextKey{}).(Conn)
	if !ok {
		return fmt.Errorf("db: savepoint: no transaction in context")
	}
	sp := fmt.Sprintf("sp_%d", globalSpID.Add(1))

	if _, err := conn.ExecContext(ctx, "SAVEPOINT "+sp); err != nil {
		return fmt.Errorf("db: savepoint %s: %w", sp, err)
	}

	if err := fn(ctx); err != nil {
		if _, rbErr := conn.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
			m.logger.Error(ctx, "db: rollback savepoint failed", "savepoint", sp, "error", rbErr)
		}
		return err
	}

	if _, err := conn.ExecContext(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
		return fmt.Errorf("db: release savepoint %s: %w", sp, err)
	}
	return nil
}
