package schedule

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sync"
)

// PgLeaderElector uses PostgreSQL advisory locks for distributed leader election.
//
// PostgreSQL advisory locks are session-level (per-connection). Because *sql.DB
// is a connection pool, TryAcquire and Release may execute on different pooled
// connections, which would cause Release to silently fail. To solve this,
// PgLeaderElector acquires a dedicated *sql.Conn for each lock and stores it in
// a map keyed by the original string key so that Release can unlock on the exact
// same connection.
type PgLeaderElector struct {
	db    *sql.DB
	mu    sync.Mutex
	locks map[string]*lockEntry
}

// lockEntry stores the dedicated connection and advisory lock ID for a single lock.
// refCount tracks re-entrant TryAcquire calls; the underlying advisory lock is
// only released when refCount reaches zero.
type lockEntry struct {
	conn     *sql.Conn
	id       int64
	refCount int
}

// NewPgLeaderElector creates a PgLeaderElector backed by the given database.
func NewPgLeaderElector(db *sql.DB) *PgLeaderElector {
	return &PgLeaderElector{
		db:    db,
		locks: make(map[string]*lockEntry),
	}
}

// TryAcquire attempts to acquire a PostgreSQL advisory lock for the given key.
// It returns true if the lock was acquired, false if it is already held by
// another connection. The key string is hashed to an int64 via FNV-1a.
func (e *PgLeaderElector) TryAcquire(ctx context.Context, key string) (bool, error) {
	e.mu.Lock()
	if entry, held := e.locks[key]; held {
		// Already held by us; increment reference count for re-entrant callers.
		entry.refCount++
		e.mu.Unlock()
		return true, nil
	}
	e.mu.Unlock()

	id := hashKey(key)

	// Obtain a dedicated connection from the pool so the advisory lock is
	// bound to a specific session.
	conn, err := e.db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("schedule: pg leader: %w", err)
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", id).Scan(&acquired); err != nil {
		conn.Close()
		return false, fmt.Errorf("schedule: pg leader: %w", err)
	}

	if !acquired {
		// Lock is held by another session; release the dedicated connection
		// back to the pool immediately.
		conn.Close()
		return false, nil
	}

	e.mu.Lock()
	e.locks[key] = &lockEntry{conn: conn, id: id, refCount: 1}
	e.mu.Unlock()

	return true, nil
}

// Release unlocks the PostgreSQL advisory lock for the given key. If the key
// was never acquired (or was already released), Release is a no-op and returns
// nil to support idempotent usage.
func (e *PgLeaderElector) Release(ctx context.Context, key string) error {
	e.mu.Lock()
	entry, ok := e.locks[key]
	if !ok {
		e.mu.Unlock()
		return nil
	}
	entry.refCount--
	if entry.refCount > 0 {
		// Other callers still hold a reference; keep the advisory lock.
		e.mu.Unlock()
		return nil
	}
	delete(e.locks, key)
	e.mu.Unlock()

	var unlocked bool
	err := entry.conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", entry.id).Scan(&unlocked)

	// Always close the dedicated connection, regardless of the unlock result.
	entry.conn.Close()

	if err != nil {
		return fmt.Errorf("schedule: pg leader: %w", err)
	}

	return nil
}

// Close releases all held advisory locks and closes their dedicated
// connections. It is safe to call Close multiple times.
func (e *PgLeaderElector) Close(ctx context.Context) error {
	e.mu.Lock()
	locks := e.locks
	e.locks = make(map[string]*lockEntry)
	e.mu.Unlock()

	var firstErr error
	for _, entry := range locks {
		if err := entry.conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", entry.id).Err(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("schedule: pg leader: %w", err)
		}
		entry.conn.Close()
	}
	return firstErr
}

// compile-time interface check
var _ LeaderElector = (*PgLeaderElector)(nil)

// hashKey converts a string key to an int64 using FNV-1a. PostgreSQL
// pg_try_advisory_lock accepts a bigint, so this mapping lets callers use
// human-readable names like "schedule:daily-cleanup".
//
// NOTE: FNV-1a 64-bit has a non-zero (though very low) probability of hash
// collisions. Two unrelated keys that hash to the same int64 will contend for
// the same advisory lock. For safety, keep lock key names centralized (e.g., as
// package-level constants) and avoid user-generated dynamic keys. If stronger
// guarantees are needed, consider using the two-argument form of
// pg_try_advisory_lock(classid, objid) with an application-assigned namespace.
func hashKey(key string) int64 {
	h := fnv.New64a()
	// fnv hash.Write never returns an error.
	h.Write([]byte(key))
	return int64(h.Sum64())
}
