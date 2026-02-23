package broth

import (
	"context"

	"github.com/source-maker/broth/admin"
	"github.com/source-maker/broth/job"
	"github.com/source-maker/broth/router"
	"github.com/source-maker/broth/schedule"
)

// Module is the core interface that every Broth module must implement.
type Module interface {
	// Name returns the unique module identifier (e.g., "account", "article").
	Name() string

	// Routes returns the routes to register for this module.
	Routes() []router.Route
}

// Migrator is an optional interface for modules that provide database migrations.
type Migrator interface {
	MigrationsDir() string
}

// JobProvider is an optional interface for modules that define background jobs.
type JobProvider interface {
	Jobs() []job.Definition
}

// ScheduleProvider is an optional interface for modules that define scheduled jobs.
type ScheduleProvider interface {
	Schedules() []schedule.Definition
}

// AdminProvider is an optional interface for modules that register admin panel resources.
type AdminProvider interface {
	AdminResources() []admin.Resource
}

// OnStart is an optional interface for modules that need initialization on app start.
type OnStart interface {
	Start(ctx context.Context) error
}

// OnShutdown is an optional interface for modules with graceful shutdown logic.
type OnShutdown interface {
	Shutdown(ctx context.Context) error
}
