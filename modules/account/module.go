package account

import (
	"github.com/source-maker/broth"
	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/log"
	"github.com/source-maker/broth/render"
)

// Module is the account module for user registration, login, and profile.
type Module struct {
	handler *Handler
	service *Service
}

// NewModule creates an account Module with the given dependencies.
func NewModule(repo Repository, hasher auth.PasswordHasher, tokenSvc *auth.TokenService, renderer *render.Renderer, logger *log.Logger) *Module {
	svc := NewService(repo, hasher, logger)
	svc.tokenService = tokenSvc
	handler := NewHandler(svc, renderer)
	return &Module{
		handler: handler,
		service: svc,
	}
}

// Name returns the module identifier.
func (m *Module) Name() string { return "account" }

// Service returns the account service for use by other modules.
func (m *Module) Service() *Service { return m.service }

// Compile-time interface check.
var _ broth.Module = (*Module)(nil)
