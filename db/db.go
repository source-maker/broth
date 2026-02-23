// Package db provides database connection management and transaction support.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/source-maker/broth/log"
)

// Config holds database connection parameters.
type Config struct {
	URL             string        `env:"DATABASE_URL" required:"true"`
	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" default:"30m"`
	ConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME" default:"5m"`
}

// DefaultConfig returns a Config with sensible defaults for the given URL.
func DefaultConfig(url string) Config {
	return Config{
		URL:             url,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Database manages the database connection pool and provides transaction support.
type Database struct {
	db     *sql.DB
	logger *log.Logger
	txMgr  *TxMgr
}

// Open connects to the database using the provided configuration.
func Open(cfg Config, logger *log.Logger) (*Database, error) {
	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	d := &Database{db: db, logger: logger}
	d.txMgr = NewTxManager(db, logger)
	return d, nil
}

// MustOpen calls Open and panics on error.
func MustOpen(cfg Config, logger *log.Logger) *Database {
	d, err := Open(cfg, logger)
	if err != nil {
		panic(err)
	}
	return d
}

// DB returns the underlying *sql.DB for direct access.
func (d *Database) DB() *sql.DB {
	return d.db
}

// TxManager returns the TxMgr bound to this database.
// The same instance is returned on every call to ensure savepoint IDs
// are unique across the entire database lifecycle.
func (d *Database) TxManager() *TxMgr {
	return d.txMgr
}

// Close closes the database connection pool.
func (d *Database) Close() error {
	return d.db.Close()
}

// HealthCheck verifies the database is reachable.
func (d *Database) HealthCheck(ctx context.Context) error {
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("db: health check: %w", err)
	}
	return nil
}
