package account

import (
	"net/http"

	"github.com/source-maker/broth/middleware"
	"github.com/source-maker/broth/router"
)

// Routes returns the route definitions for the account module.
func (m *Module) Routes() []router.Route {
	requireAuth := middleware.RequireAuth()

	return []router.Route{
		// SSR routes
		{Pattern: "GET /login", Handler: http.HandlerFunc(m.handler.ShowLogin)},
		{Pattern: "POST /login", Handler: http.HandlerFunc(m.handler.Login)},
		{Pattern: "GET /register", Handler: http.HandlerFunc(m.handler.ShowRegister)},
		{Pattern: "POST /register", Handler: http.HandlerFunc(m.handler.Register)},
		{Pattern: "POST /logout", Handler: http.HandlerFunc(m.handler.Logout)},
		{Pattern: "GET /profile", Handler: requireAuth(http.HandlerFunc(m.handler.Profile))},

		// API routes
		{Pattern: "POST /api/login", Handler: middleware.ForceContext(middleware.ContextAPI,
			http.HandlerFunc(m.handler.APILogin))},
		{Pattern: "GET /api/me", Handler: middleware.ForceContext(middleware.ContextAPI,
			requireAuth(http.HandlerFunc(m.handler.APICurrentUser)))},
	}
}
