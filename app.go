package broth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/source-maker/broth/log"
	"github.com/source-maker/broth/router"
)

// App is the top-level Broth application container.
// It manages module registration, route mounting, and lifecycle.
type App struct {
	modules []Module
	router  *router.Router
	logger  *log.Logger
}

// New creates a new App with the given logger.
func New(logger *log.Logger) *App {
	return &App{
		router: router.New(),
		logger: logger,
	}
}

// Register adds modules to the application.
func (a *App) Register(modules ...Module) {
	a.modules = append(a.modules, modules...)
}

// Mount mounts all registered module routes onto the router.
// Each module's routes are prefixed with /{moduleName}.
func (a *App) Mount() {
	for _, mod := range a.modules {
		prefix := "/" + mod.Name()
		a.router.Mount(prefix, mod.Routes())
		a.logger.Info(nil, "module mounted", "module", mod.Name(), "prefix", prefix)
	}
}

// Use adds middleware to the application router.
func (a *App) Use(mw ...router.Middleware) {
	a.router.Use(mw...)
}

// Handler returns the application's http.Handler with all middleware applied.
func (a *App) Handler() http.Handler {
	return a.router.Handler()
}

// Start initializes all modules that implement OnStart.
func (a *App) Start(ctx context.Context) error {
	for _, mod := range a.modules {
		if starter, ok := mod.(OnStart); ok {
			if err := starter.Start(ctx); err != nil {
				return fmt.Errorf("broth: start %s: %w", mod.Name(), err)
			}
			a.logger.Info(ctx, "module started", "module", mod.Name())
		}
	}
	return nil
}

// Shutdown gracefully shuts down all modules that implement OnShutdown.
func (a *App) Shutdown(ctx context.Context) error {
	var firstErr error
	for _, mod := range a.modules {
		if stopper, ok := mod.(OnShutdown); ok {
			if err := stopper.Shutdown(ctx); err != nil {
				a.logger.Error(ctx, "module shutdown failed", "module", mod.Name(), "error", err)
				if firstErr == nil {
					firstErr = fmt.Errorf("broth: shutdown %s: %w", mod.Name(), err)
				}
			}
		}
	}
	return firstErr
}

// Modules returns all registered modules.
func (a *App) Modules() []Module {
	return a.modules
}

// Router returns the underlying router for advanced configuration.
func (a *App) Router() *router.Router {
	return a.router
}
