// Package testutil provides test helpers for Broth applications.
package testutil

import (
	"database/sql"
	"os"
	"testing"
)

// SetupTestDB creates a test database connection using the TEST_DATABASE_URL
// environment variable and registers cleanup.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("testutil: open db: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("testutil: ping db: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}
