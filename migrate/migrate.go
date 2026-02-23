// Package migrate provides database migration management.
// Wraps goose for SQL-file-based schema migrations.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
)

// MigrationStatus represents the state of a single migration.
type MigrationStatus struct {
	Version   int64
	Name      string
	AppliedAt time.Time
	Applied   bool
}

// Migrator manages database schema migrations.
type Migrator struct {
	db        *sql.DB
	sourceDir string
}

// New creates a Migrator for the given database and migration source directory.
func New(db *sql.DB, sourceDir string) *Migrator {
	return &Migrator{db: db, sourceDir: sourceDir}
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	if err := goose.RunContext(ctx, "up", m.db, m.sourceDir); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}

// Down rolls back the last applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if err := goose.RunContext(ctx, "down", m.db, m.sourceDir); err != nil {
		return fmt.Errorf("migrate: down: %w", err)
	}
	return nil
}

// Status returns the status of all known migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	migrations, err := goose.CollectMigrations(m.sourceDir, 0, goose.MaxVersion)
	if err != nil {
		return nil, fmt.Errorf("migrate: collect: %w", err)
	}

	dbMigrations, err := goose.GetDBVersionContext(ctx, m.db)
	if err != nil {
		return nil, fmt.Errorf("migrate: db version: %w", err)
	}

	var statuses []MigrationStatus
	for _, mg := range migrations {
		s := MigrationStatus{
			Version: mg.Version,
			Name:    mg.Source,
			Applied: mg.Version <= dbMigrations,
		}
		statuses = append(statuses, s)
	}

	return statuses, nil
}
