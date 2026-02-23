package account

import (
	"context"
	"fmt"
	"time"

	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/log"
)

// Service contains the business logic for account management.
type Service struct {
	repo         Repository
	hasher       auth.PasswordHasher
	logger       *log.Logger
	tokenService *auth.TokenService
}

// NewService creates an account Service.
func NewService(repo Repository, hasher auth.PasswordHasher, logger *log.Logger) *Service {
	return &Service{
		repo:   repo,
		hasher: hasher,
		logger: logger,
	}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*User, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Check if email already exists
	existing, _ := s.repo.FindByEmail(ctx, input.Email)
	if existing != nil {
		return nil, fmt.Errorf("account: register: email already in use")
	}

	hash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("account: register: %w", err)
	}

	now := time.Now()
	user := &User{
		Email:        input.Email,
		Name:         input.Name,
		PasswordHash: hash,
		Roles:        []string{auth.RoleViewer},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("account: register: %w", err)
	}

	s.logger.Info(ctx, "user registered", "user_id", user.ID, "email", user.Email)
	return user, nil
}

// Authenticate verifies credentials and returns the user.
func (s *Service) Authenticate(ctx context.Context, input LoginInput) (*User, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	user, err := s.repo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("account: authenticate: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("account: authenticate: invalid credentials")
	}

	if err := s.hasher.Verify(input.Password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("account: authenticate: invalid credentials")
	}

	s.logger.Info(ctx, "user authenticated", "user_id", user.ID)
	return user, nil
}

// FindByID retrieves a user by ID.
func (s *Service) FindByID(ctx context.Context, id int64) (*User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("account: find: %w", err)
	}
	return user, nil
}
