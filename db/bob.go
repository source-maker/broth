package db

import (
	"context"
	"database/sql"

	"github.com/stephenafamo/bob"
)

// BobDB returns a Bob-wrapped database handle.
// The returned bob.DB implements bob.Executor and can be passed
// to repository constructors for type-safe query execution.
func (d *Database) BobDB() bob.DB {
	return bob.NewDB(d.db)
}

// ExecutorFromContext returns a bob.Executor from the context.
// If a transaction was started by TxMgr, the *sql.Tx stored in ctx
// is wrapped with bob.NewTx and returned. Otherwise fallback is returned.
//
// Invariant: contextWithConn (called by TxMgr.runNewTx) always stores a
// *sql.Tx as the context value. This function relies on that invariant to
// produce a bob.Tx. If the stored value is not *sql.Tx (e.g. a custom Conn
// wrapper), the fallback executor is returned instead.
//
//	func (s *UserStore) Create(ctx context.Context, u *User) error {
//	    exec := db.ExecutorFromContext(ctx, s.executor)
//	    // exec is bob.Tx inside RunInTx, bob.DB otherwise
//	    ...
//	}
func ExecutorFromContext(ctx context.Context, fallback bob.Executor) bob.Executor {
	v := ctx.Value(txContextKey{})
	if v == nil {
		return fallback
	}
	if tx, ok := v.(*sql.Tx); ok {
		return bob.NewTx(tx)
	}
	return fallback
}
